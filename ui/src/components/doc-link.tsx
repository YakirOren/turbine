import { ArrowUpRight } from "lucide-react";
import { cn } from "@/lib/utils";

const DOCS_BASE_URL = "https://turbine.yakir.io/";

interface DocLinkProps {
  path?: string;
  children: React.ReactNode;
  className?: string;
}

export function DocLink({ path = "", children, className }: DocLinkProps) {
  return (
    <a
      href={`${DOCS_BASE_URL}${path}`}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        "inline-flex items-center gap-1 text-muted-foreground underline-offset-4 hover:text-foreground hover:underline",
        className,
      )}
    >
      {children}
      <ArrowUpRight className="h-3 w-3" />
    </a>
  );
}
