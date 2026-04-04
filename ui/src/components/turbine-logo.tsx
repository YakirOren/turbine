import type { SVGProps } from "react";

export function TurbineLogo(props: SVGProps<SVGSVGElement>) {
  // Single blade path: a teardrop/petal shape extending upward from center
  const blade = "M50 44 C42 28, 38 12, 50 0 C62 12, 58 28, 50 44Z";

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 100 100"
      fill="currentColor"
      {...props}
    >
      <path d={blade} />
      <path d={blade} transform="rotate(120 50 50)" />
      <path d={blade} transform="rotate(240 50 50)" />
      <circle cx="50" cy="50" r="8" />
    </svg>
  );
}
