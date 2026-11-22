package document

import (
	"bytes"
	"image"
	"image/png"
	"sort"
	"strings"

	"golang.org/x/image/ccitt"
)

const (
	// 单张页面图片的输出上限，与 AI 视觉输入保持一致。
	maxExtractedImageBytes  = 10 << 20
	maxExtractedImageEdge   = 20000
	maxExtractedImagePixels = 60_000_000
	// 需要解码为像素的图片更严格，避免 Flate/CCITT 大图占用过多内存。
	maxDecodedImagePixels = 25_000_000
	// 过滤 logo、印章等小图，只保留接近整页的扫描图。
	minExtractedImageEdge = 200
	minExtractedImageArea = 120_000
	maxPageImages         = 3
)

// PDFImage 是 PDF 页面内嵌的可直接交给视觉模型的位图。
type PDFImage struct {
	Data   []byte
	MIME   string
	Width  int
	Height int
	Page   int
}

// ExtractPDFImages 提取 PDF 页面内嵌的位图，供不接受文件输入的多模态模型直接阅读。
// 只处理 DCT/Flate/CCITT 等常见扫描编码，不做页面渲染和 OCR。
func ExtractPDFImages(data []byte) ([]PDFImage, []string) {
	doc := buildPDFDocument(data)
	if len(doc.objects) == 0 {
		return nil, []string{"PDF 结构无法识别"}
	}
	pages := doc.pages(nil)
	images := make([]PDFImage, 0)
	seen := make(map[int]bool)
	if len(pages) > 0 {
		for _, page := range pages {
			pageImages := make([]PDFImage, 0, len(page.images))
			for _, reference := range page.images {
				if seen[reference] {
					continue
				}
				seen[reference] = true
				if extracted, ok := doc.pdfImage(reference, page.number); ok {
					pageImages = append(pageImages, extracted)
				}
			}
			images = append(images, largestPageImages(pageImages)...)
		}
	} else {
		for _, number := range doc.order {
			object := doc.objects[number]
			if object == nil {
				continue
			}
			if pdfDictName(doc.objectDict(object), "Subtype") != "Image" {
				continue
			}
			if extracted, ok := doc.pdfImage(number, 0); ok {
				images = append(images, extracted)
			}
		}
	}
	if len(images) == 0 {
		return nil, []string{"未找到可用的页面图片"}
	}
	sort.SliceStable(images, func(i, j int) bool {
		if images[i].Page != images[j].Page {
			return images[i].Page < images[j].Page
		}
		return images[i].Width*images[i].Height > images[j].Width*images[j].Height
	})
	return images, nil
}

// largestPageImages 只保留接近整页的图片，避免把页眉 logo、印章也发给模型。
func largestPageImages(images []PDFImage) []PDFImage {
	if len(images) <= 1 {
		return images
	}
	sort.SliceStable(images, func(i, j int) bool {
		return images[i].Width*images[i].Height > images[j].Width*images[j].Height
	})
	largest := images[0].Width * images[0].Height
	kept := make([]PDFImage, 0, maxPageImages)
	for _, item := range images {
		if item.Width*item.Height*2 < largest {
			break
		}
		kept = append(kept, item)
		if len(kept) == maxPageImages {
			break
		}
	}
	return kept
}

func (d *pdfDocument) pdfImage(number, page int) (PDFImage, bool) {
	object := d.objects[number]
	if object == nil {
		return PDFImage{}, false
	}
	dictionary, raw, ok := splitPDFStream(object.body)
	if !ok {
		return PDFImage{}, false
	}
	dict := parsePDFDict(dictionary)
	width := pdfDictInt(dict, "Width", 0)
	height := pdfDictInt(dict, "Height", 0)
	if !usablePDFImageSize(width, height) || pdfDictBool(dict, "ImageMask") {
		return PDFImage{}, false
	}
	params := dict
	if nested, ok := d.dictValue(dict, "DecodeParms"); ok {
		params = nested
	}
	data := raw
	filters := pdfFilterNames(dictionary)
	for index, filter := range filters {
		switch filter {
		case "ASCIIHexDecode", "AHx":
			decoded, err := decodePDFASCIIHex(data)
			if err != nil {
				return PDFImage{}, false
			}
			data = decoded
		case "ASCII85Decode", "A85":
			decoded, err := decodePDFASCII85(data)
			if err != nil {
				return PDFImage{}, false
			}
			data = decoded
		case "FlateDecode", "Fl":
			data = inflatePDFStream(data)
			if data == nil {
				return PDFImage{}, false
			}
		case "RunLengthDecode", "RL":
			data = decodePDFRunLength(data)
			if len(data) == 0 {
				return PDFImage{}, false
			}
		case "DCTDecode", "DCT":
			if index != len(filters)-1 || len(data) == 0 || len(data) > maxExtractedImageBytes {
				return PDFImage{}, false
			}
			return PDFImage{Data: data, MIME: "image/jpeg", Width: width, Height: height, Page: page}, true
		case "CCITTFaxDecode", "CCF":
			return decodeCCITTPDFImage(data, params, width, height, page)
		default:
			return PDFImage{}, false
		}
	}
	return encodePDFPixels(dict, params, data, width, height, page)
}

func usablePDFImageSize(width, height int) bool {
	if width < minExtractedImageEdge || height < minExtractedImageEdge {
		return false
	}
	if width > maxExtractedImageEdge || height > maxExtractedImageEdge {
		return false
	}
	pixels := int64(width) * int64(height)
	return pixels >= minExtractedImageArea && pixels <= maxExtractedImagePixels
}

func decodeCCITTPDFImage(data []byte, params map[string][]byte, width, height, page int) (PDFImage, bool) {
	if int64(width)*int64(height) > maxDecodedImagePixels {
		return PDFImage{}, false
	}
	subFormat := ccitt.Group3
	if pdfDictInt(params, "K", 0) < 0 {
		subFormat = ccitt.Group4
	}
	options := &ccitt.Options{
		Align:  pdfDictBool(params, "EncodedByteAlign"),
		Invert: pdfDictBool(params, "BlackIs1"),
	}
	target := image.NewGray(image.Rect(0, 0, width, height))
	if err := ccitt.DecodeIntoGray(target, bytes.NewReader(data), ccitt.MSB, subFormat, options); err != nil {
		return PDFImage{}, false
	}
	return encodePDFPNG(target, width, height, page)
}

func encodePDFPixels(dict, params map[string][]byte, data []byte, width, height, page int) (PDFImage, bool) {
	if pdfDictInt(params, "Predictor", 1) > 1 {
		return PDFImage{}, false
	}
	if int64(width)*int64(height) > maxDecodedImagePixels {
		return PDFImage{}, false
	}
	bits := pdfDictInt(dict, "BitsPerComponent", 8)
	switch bits {
	case 1:
		return encodePDFBitPixels(data, width, height, page)
	case 8:
		return encodePDFBytePixels(dict, data, width, height, page)
	default:
		return PDFImage{}, false
	}
}

func encodePDFBitPixels(data []byte, width, height, page int) (PDFImage, bool) {
	stride := (width + 7) / 8
	if len(data) < stride*height {
		return PDFImage{}, false
	}
	target := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		row := data[y*stride : (y+1)*stride]
		for x := 0; x < width; x++ {
			value := byte(0)
			if row[x/8]&(0x80>>uint(x%8)) != 0 {
				value = 0xFF
			}
			target.Pix[y*target.Stride+x] = value
		}
	}
	return encodePDFPNG(target, width, height, page)
}

func encodePDFBytePixels(dict map[string][]byte, data []byte, width, height, page int) (PDFImage, bool) {
	channels := pdfColorChannels(dict, len(data), width, height)
	switch channels {
	case 1:
		if len(data) < width*height {
			return PDFImage{}, false
		}
		target := image.NewGray(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			copy(target.Pix[y*target.Stride:y*target.Stride+width], data[y*width:(y+1)*width])
		}
		return encodePDFPNG(target, width, height, page)
	case 3:
		if len(data) < width*height*3 {
			return PDFImage{}, false
		}
		target := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			source := data[y*width*3 : (y+1)*width*3]
			row := target.Pix[y*target.Stride : y*target.Stride+width*4]
			for x := 0; x < width; x++ {
				row[x*4] = source[x*3]
				row[x*4+1] = source[x*3+1]
				row[x*4+2] = source[x*3+2]
				row[x*4+3] = 0xFF
			}
		}
		return encodePDFPNG(target, width, height, page)
	case 4:
		if len(data) < width*height*4 {
			return PDFImage{}, false
		}
		target := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			source := data[y*width*4 : (y+1)*width*4]
			row := target.Pix[y*target.Stride : y*target.Stride+width*4]
			for x := 0; x < width; x++ {
				c, m, yellow, k := int(source[x*4]), int(source[x*4+1]), int(source[x*4+2]), int(source[x*4+3])
				row[x*4] = uint8((255 - c) * (255 - k) / 255)
				row[x*4+1] = uint8((255 - m) * (255 - k) / 255)
				row[x*4+2] = uint8((255 - yellow) * (255 - k) / 255)
				row[x*4+3] = 0xFF
			}
		}
		return encodePDFPNG(target, width, height, page)
	default:
		return PDFImage{}, false
	}
}

func pdfColorChannels(dict map[string][]byte, length, width, height int) int {
	switch strings.ToLower(pdfDictName(dict, "ColorSpace")) {
	case "devicergb", "rgb", "calrgb":
		return 3
	case "devicegray", "g", "calgray":
		return 1
	case "devicecmyk", "cmyk":
		return 4
	}
	if width <= 0 || height <= 0 {
		return 0
	}
	pixels := width * height
	switch {
	case length >= pixels*4:
		return 4
	case length >= pixels*3:
		return 3
	case length >= pixels:
		return 1
	}
	return 0
}

func encodePDFPNG(source image.Image, width, height, page int) (PDFImage, bool) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, source); err != nil {
		return PDFImage{}, false
	}
	if buffer.Len() > maxExtractedImageBytes {
		return PDFImage{}, false
	}
	return PDFImage{Data: buffer.Bytes(), MIME: "image/png", Width: width, Height: height, Page: page}, true
}

func pdfDictBool(dictionary map[string][]byte, key string) bool {
	return strings.EqualFold(strings.TrimSpace(string(dictionary[key])), "true")
}
