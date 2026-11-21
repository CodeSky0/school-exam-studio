import {
  BarChartIcon,
  CheckCircledIcon,
  CopyIcon,
  DashboardIcon,
  EyeOpenIcon,
  FileTextIcon,
  LayersIcon,
  LightningBoltIcon,
  LockClosedIcon,
  LoopIcon,
  MagicWandIcon,
  ReaderIcon,
  TableIcon,
} from "@radix-ui/react-icons";

// SkillIcon 统一映射后端技能的 icon 字段，避免各面板重复维护同一张表。
export function SkillIcon({ icon, className = "size-4" }: { icon: string; className?: string }) {
  switch (icon) {
    case "file": return <FileTextIcon className={className} />;
    case "book": return <ReaderIcon className={className} />;
    case "split": return <MagicWandIcon className={className} />;
    case "shield": return <LockClosedIcon className={className} />;
    case "spark": return <LightningBoltIcon className={className} />;
    case "paper": return <TableIcon className={className} />;
    case "repeat": return <LoopIcon className={className} />;
    case "chart": return <BarChartIcon className={className} />;
    case "copy": return <CopyIcon className={className} />;
    case "check": return <CheckCircledIcon className={className} />;
    case "eye": return <EyeOpenIcon className={className} />;
    case "table": return <DashboardIcon className={className} />;
    case "layers": return <LayersIcon className={className} />;
    default: return <MagicWandIcon className={className} />;
  }
}