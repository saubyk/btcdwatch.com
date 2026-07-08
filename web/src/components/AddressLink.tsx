/**
 * True when the string is a real address rather than a backend
 * placeholder like "(non-standard output)". Matched by shape — no
 * address encoding starts with "(" — so backend copy edits can't turn a
 * placeholder into a link.
 */
export function isLinkableAddress(addr: string | undefined): addr is string {
  return !!addr && !addr.startsWith('(')
}

/**
 * A tappable address: navigates to the Address view via the existing
 * search path (loading skeleton included). Non-standard placeholders
 * render as plain text. `display` truncates the label while the full
 * address travels with the click.
 */
export function AddressLink({
  address,
  display,
  breakAll = false,
  onSearch,
}: {
  address: string
  display?: string
  breakAll?: boolean
  onSearch: (q: string) => void
}) {
  const cls = `bp-addr-link${breakAll ? ' bp-addr-link--break' : ''}`
  if (!isLinkableAddress(address)) {
    return <span className={breakAll ? 'bp-addr-link--break' : undefined}>{display ?? address}</span>
  }
  return (
    <button className={cls} onClick={() => onSearch(address)} title="View this address">
      {display ?? address}
    </button>
  )
}
