import type { ReactNode } from "react";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { useMediaQuery } from "@/lib/use-media-query";
import { cn } from "@/lib/utils";

export function DrawerShell({
    width = "w-[340px]",
    sheetClassName,
    srLabel,
    onClose,
    children,
}: {
    width?: string;
    sheetClassName?: string;
    srLabel: string;
    onClose: () => void;
    children: ReactNode;
}) {
    const isLg = useMediaQuery("(min-width: 1024px)");
    if (isLg) {
        return (
            <aside
                className={cn(
                    "flex shrink-0 flex-col overflow-hidden border-l bg-card animate-in fade-in slide-in-from-right-8 duration-500 ease-out",
                    width
                )}
            >
                {children}
            </aside>
        );
    }
    return (
        <Sheet open onOpenChange={(o) => !o && onClose()}>
            <SheetContent
                side="right"
                className={cn("w-full sm:max-w-sm overflow-y-auto p-0 bg-card", sheetClassName)}
            >
                <SheetTitle className="sr-only">{srLabel}</SheetTitle>
                {children}
            </SheetContent>
        </Sheet>
    );
}
