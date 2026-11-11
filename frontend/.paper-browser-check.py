import io
import json
from decimal import Decimal
from urllib.parse import urlparse, parse_qs
from playwright.sync_api import sync_playwright

USER_ID = "c2dd2e3f-b0c6-4b0c-a8ce-970e7ac3cf94"
PAPER_ID = "47d1f02c-bca9-45b2-a6b1-c9a42971f97e"
Q1 = "1f08b234-0a69-4a1f-a2f5-f8a24f2f77a1"
Q2 = "ba2dd0d1-97cf-4f52-9597-dc87a85a77a3"

user = {
    "id": USER_ID,
    "school_id": "0f07a25b-e8b7-4fc2-969f-c0740ac528f4",
    "name": "周老师",
    "subject": "数学",
    "email": "zhou@example.edu.cn",
    "role": "teacher",
    "status": "active",
}
questions = [
    {
        "id": Q1,
        "creator_id": USER_ID,
        "creator_name": "周老师",
        "subject": "数学",
        "grade": "八年级",
        "question_type": "single_choice",
        "difficulty": 2,
        "tags": ["方程"],
        "content": '<p>方程 <span data-type="inline-math" data-latex="x^2=4"></span> 的正根是</p>',
        "options": [
            {"key": "A", "content": "<p>2</p>"},
            {"key": "B", "content": "<p>4</p>"},
        ],
        "answer": "A",
        "explanation": "<p>正数平方根为 2。</p>",
        "status": "published",
        "created_at": "2026-11-10T08:00:00Z",
        "updated_at": "2026-11-10T08:00:00Z",
    },
    {
        "id": Q2,
        "creator_id": USER_ID,
        "creator_name": "周老师",
        "subject": "数学",
        "grade": "八年级",
        "question_type": "essay",
        "difficulty": 4,
        "tags": ["几何"],
        "content": "<p>说明勾股定理，并给出一个实际测量中的应用。</p>",
        "options": [],
        "answer": "<p>参考答案不应出现在学生卷面。</p>",
        "explanation": "<p>解析不应出现在学生卷面。</p>",
        "status": "published",
        "created_at": "2026-11-10T08:00:00Z",
        "updated_at": "2026-11-10T08:00:00Z",
    },
]
question_map = {question["id"]: question for question in questions}


def make_paper(payload):
    items = []
    total = Decimal("0")
    for index, item in enumerate(payload["questions"]):
        score = Decimal(item["assigned_score"])
        total += score
        items.append({
            "question_id": item["question_id"],
            "order_index": index,
            "assigned_score": item["assigned_score"],
            "question": question_map[item["question_id"]],
        })
    return {
        "id": PAPER_ID,
        "creator_id": USER_ID,
        "school_name": "明德实验中学",
        "title": payload["title"],
        "subject": payload["subject"],
        "grade": payload["grade"],
        "total_score": f"{total:.2f}",
        "duration_minutes": payload["duration_minutes"],
        "questions": items,
        "created_at": "2026-11-10T08:00:00Z",
        "updated_at": "2026-11-10T08:00:00Z",
    }


state = {"paper": None, "last_post": None, "question_queries": []}


def handle_api(route):
    request = route.request
    headers = {
        "Access-Control-Allow-Origin": "http://localhost:3000",
        "Access-Control-Allow-Credentials": "true",
        "Access-Control-Allow-Headers": "Content-Type",
        "Access-Control-Allow-Methods": "GET,POST,PATCH,DELETE,OPTIONS",
    }
    if request.method == "OPTIONS":
        route.fulfill(status=204, headers=headers, body="")
        return
    path = urlparse(request.url).path
    query = parse_qs(urlparse(request.url).query)
    response_headers = {**headers, "Content-Type": "application/json"}

    def respond(payload, status=200):
        route.fulfill(status=status, headers=response_headers, body=json.dumps(payload, ensure_ascii=False))

    if path == "/api/v1/auth/me":
        respond({"user": user})
        return
    if path == "/api/v1/questions/resolve":
        ids = json.loads(request.post_data or "{}")["ids"]
        respond({"items": [question_map[item] for item in ids if item in question_map]})
        return
    if path == "/api/v1/questions":
        state["question_queries"].append(query)
        respond({"items": questions, "total": len(questions), "page": 1, "page_size": 6})
        return
    if path == "/api/v1/papers" and request.method == "GET":
        items = [state["paper"]] if state["paper"] else []
        respond({"items": items, "total": len(items), "page": 1, "page_size": 10})
        return
    if path == "/api/v1/papers" and request.method == "POST":
        payload = json.loads(request.post_data)
        state["last_post"] = payload
        state["paper"] = make_paper(payload)
        respond(state["paper"], 201)
        return
    if path == f"/api/v1/papers/{PAPER_ID}" and request.method == "GET":
        if state["paper"]:
            respond(state["paper"])
        else:
            respond({"error": {"code": "PAPER_NOT_FOUND", "message": "试卷不存在"}, "request_id": "browser-check"}, 404)
        return
    if path == f"/api/v1/papers/{PAPER_ID}" and request.method == "PATCH":
        payload = json.loads(request.post_data)
        state["paper"] = make_paper(payload)
        respond(state["paper"])
        return
    if path == f"/api/v1/papers/{PAPER_ID}" and request.method == "DELETE":
        state["paper"] = None
        route.fulfill(status=204, headers=headers, body="")
        return
    respond({"error": {"code": "NOT_FOUND", "message": "未配置测试路由"}, "request_id": "browser-check"}, 404)


with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True)
    page = browser.new_page(viewport={"width": 1600, "height": 1100})
    console_errors = []
    page_errors = []
    page.on("console", lambda message: console_errors.append(message.text) if message.type == "error" else None)
    page.on("pageerror", lambda error: page_errors.append(str(error)))
    page.route("**/api/v1/**", handle_api)
    page.goto("http://localhost:3000/teacher/papers")
    page.wait_for_load_state("networkidle")
    page.get_by_role("heading", name="组卷中心", exact=True).wait_for()
    assert page.get_by_role("button", name="加入试卷", exact=True).count() == 2
    assert all(query.get("status") == ["published"] for query in state["question_queries"])

    page.get_by_role("button", name="加入试卷", exact=True).nth(0).click()
    page.get_by_role("button", name="加入试卷", exact=True).nth(0).click()
    page.locator(f"#cart-score-{Q1}").fill("0.10")
    page.locator(f"#cart-score-{Q2}").fill("0.20")
    page.wait_for_function("() => document.querySelector('#paper-cart-title')?.closest('section')?.innerText.includes('0.30')")
    storage_keys = page.evaluate("Object.keys(localStorage).filter((key) => key.startsWith('paper-cart:'))")
    assert storage_keys == [f"paper-cart:{USER_ID}"]

    page.evaluate("""
      () => {
        const dataTransfer = new DataTransfer();
        const source = document.querySelector('button[aria-label="拖动第 1 题排序"]');
        const target = document.querySelectorAll('ol[aria-label^="已选题目"] > li')[1];
        source.dispatchEvent(new DragEvent('dragstart', { bubbles: true, dataTransfer }));
        target.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer }));
        target.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer }));
        source.dispatchEvent(new DragEvent('dragend', { bubbles: true, dataTransfer }));
      }
    """)
    first_row = page.locator('ol[aria-label^="已选题目"] > li').nth(0)
    assert "勾股定理" in first_row.inner_text()
    page.get_by_role("button", name="上移第 2 题", exact=True).click()
    assert "正根" in page.locator('ol[aria-label^="已选题目"] > li').nth(0).inner_text()

    page.reload()
    page.wait_for_load_state("networkidle")
    page.locator(f"#cart-score-{Q1}").wait_for()
    assert page.locator(f"#cart-score-{Q1}").input_value() == "0.10"
    assert page.locator(f"#cart-score-{Q2}").input_value() == "0.20"
    page.locator("#paper-title").fill("八年级数学阶段练习")
    page.locator("#paper-grade").fill("八年级")
    page.locator("#paper-duration").fill("90")
    page.screenshot(path="/workspace/frontend/.paper-center-check.png", full_page=True)
    page.get_by_role("button", name="创建试卷", exact=True).click()
    page.wait_for_url(f"**/teacher/papers/{PAPER_ID}")
    page.get_by_role("heading", name="编辑与预览试卷", exact=True).wait_for()
    page.wait_for_load_state("networkidle")
    assert state["last_post"]["questions"] == [
        {"question_id": Q1, "assigned_score": "0.10"},
        {"question_id": Q2, "assigned_score": "0.20"},
    ]
    assert all("order_index" not in item for item in state["last_post"]["questions"])
    preview_text = page.locator(".paper-preview-stage .paper-sheet").inner_text()
    assert "明德实验中学" in preview_text
    assert "八年级数学阶段练习" in preview_text
    assert "总分：0.30 分" in preview_text
    assert "密 封 线 内 请 勿 答 题" in preview_text
    assert "一、单选题" in preview_text and "二、简答题" in preview_text
    assert "参考答案不应出现" not in preview_text and "解析不应出现" not in preview_text
    assert page.locator(".paper-preview-stage .katex").count() > 0

    page.get_by_role("link", name="打印预览", exact=True).click()
    page.wait_for_url(f"**/teacher/papers/{PAPER_ID}/print")
    page.wait_for_load_state("networkidle")
    assert page.locator(".app-shell-sidebar").count() == 0
    assert page.locator(".paper-sheet").count() == 1
    assert page.locator(".paper-sheet .katex").count() > 0
    page.screenshot(path="/workspace/frontend/.paper-print-check.png", full_page=True)
    page.emulate_media(media="print")
    assert page.locator(".print-toolbar").evaluate("(element) => getComputedStyle(element).display") == "none"
    assert page.locator(".paper-sheet").evaluate("(element) => getComputedStyle(element).display") != "none"
    pdf = page.pdf(format="A4", print_background=True)
    assert pdf.startswith(b"%PDF")
    try:
        from pypdf import PdfReader
        assert len(PdfReader(io.BytesIO(pdf)).pages) >= 1
    except ImportError:
        pass
    assert not page_errors, page_errors
    assert not console_errors, console_errors
    print(json.dumps({
        "center": "选题、去重按钮、拖拽、键盘按钮、整数百分汇总、按用户持久化、恢复均通过",
        "save": state["last_post"],
        "preview": "学校、标题、大题、分值、公式、无答案解析均通过",
        "print": "无 AppShell，打印按钮隐藏，试卷保留，A4 PDF 生成成功",
        "console_errors": console_errors,
        "page_errors": page_errors,
    }, ensure_ascii=False, indent=2))
    browser.close()
