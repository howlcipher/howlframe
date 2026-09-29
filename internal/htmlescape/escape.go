// Package htmlescape encodes the five characters that break HTML text and
// quoted attributes. html_escape and attr_escape are the same encoding under
// two names so a call site can say which context it means. Neither one is a
// JavaScript encoder, and neither one belongs inside a <script> body or an
// inline event handler.
package htmlescape

import "html"

// Escape encodes &, <, >, ", and '. The result matches html.EscapeString:
// &amp; &lt; &gt; &#34; &#39;. Replacements are not scanned again, so the
// function does not double-encode its own entities.
func Escape(s string) string {
	return html.EscapeString(s)
}
