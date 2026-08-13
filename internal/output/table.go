package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// WriteTable prints an aligned table with a header row.
func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			if i < len(row) {
				cells[i] = Cell(row[i])
			} else {
				cells[i] = "-"
			}
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

// Cell returns a table cell, using "-" for empty values.
func Cell(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// PageFooter describes how much of a paged list was shown.
// Examples: "23 items", "showing 1-25 of 87  (more: --offset 25)".
func PageFooter(offset, shown, total int) string {
	if offset < 0 {
		offset = 0
	}
	if shown < 0 {
		shown = 0
	}
	if total < offset+shown {
		total = offset + shown
	}
	if total == 0 {
		return "0 items"
	}
	if shown == 0 {
		return fmt.Sprintf("showing 0 of %d", total)
	}
	from := offset + 1
	to := offset + shown
	if offset == 0 && to >= total {
		return fmt.Sprintf("%d items", total)
	}
	s := fmt.Sprintf("showing %d-%d of %d", from, to, total)
	if to < total {
		s += fmt.Sprintf("  (more: --offset %d)", to)
	}
	return s
}
