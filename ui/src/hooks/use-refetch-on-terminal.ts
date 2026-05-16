import { useEffect, useRef } from "react";

// Holds refetch in a ref so the effect depends only on isTerminal — otherwise
// the query object's identity changes every render, the effect re-runs, and
// the cleanup clears the pending delayed refetch before it fires.
export function useRefetchOnTerminal(
  refetch: () => void,
  isTerminal: boolean,
  delayMs?: number,
) {
  const ref = useRef(refetch);
  ref.current = refetch;
  useEffect(() => {
    if (!isTerminal) return;
    ref.current();
    if (delayMs == null) return;
    const t = setTimeout(() => ref.current(), delayMs);
    return () => clearTimeout(t);
  }, [isTerminal, delayMs]);
}
