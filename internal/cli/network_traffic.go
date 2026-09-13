package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
	"github.com/SPDG/unicli/internal/output"
)

type trafficCLIFlags struct {
	start               string
	end                 string
	timezone            string
	sort                string
	top                 int
	excludeUnidentified bool
}

func newNetworkTrafficCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traffic",
		Short: "Live historical traffic reports from the selected controller",
		Long: `Read-only WAN and per-client traffic reports fetched live from the UniFi controller.

These commands query live controller APIs (not configuration mutations) and do not
require --allow-mutations. Integration v1 has no historical traffic API.

Default window is month-to-date in --timezone (or the controller sysinfo timezone).
Maximum range is 32 calendar days. network traffic clients is LAN-inclusive.
network traffic internet is Traffic Identification (Internet/DPI).`,
	}
	cmd.AddCommand(newTrafficWANCmd())
	cmd.AddCommand(newTrafficClientsCmd())
	cmd.AddCommand(newTrafficInternetCmd())
	return cmd
}

func newTrafficWANCmd() *cobra.Command {
	var f trafficCLIFlags
	cmd := &cobra.Command{
		Use:   "wan",
		Short: "Site WAN download/upload for a time range",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTraffic(cmd, f, trafficKindWAN)
		},
	}
	addTrafficFlags(cmd, &f, false)
	return cmd
}

func newTrafficClientsCmd() *cobra.Command {
	var f trafficCLIFlags
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Rank clients by historical transfer in a time range",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTraffic(cmd, f, trafficKindClients)
		},
	}
	addTrafficFlags(cmd, &f, true)
	return cmd
}

func newTrafficInternetCmd() *cobra.Command {
	var f trafficCLIFlags
	cmd := &cobra.Command{
		Use:   "internet",
		Short: "Internet/DPI site totals and client ranking for a time range",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTraffic(cmd, f, trafficKindInternet)
		},
	}
	addTrafficFlags(cmd, &f, true)
	cmd.Flags().BoolVar(&f.excludeUnidentified, "exclude-unidentified", false, "omit Unidentified DPI traffic (UI checkbox off)")
	return cmd
}

func addTrafficFlags(cmd *cobra.Command, f *trafficCLIFlags, clients bool) {
	cmd.Flags().StringVar(&f.start, "start", "", "window start (RFC3339 or YYYY-MM-DD in --timezone); default: first of month")
	cmd.Flags().StringVar(&f.end, "end", "", "window end (RFC3339 or YYYY-MM-DD inclusive date); default: now")
	cmd.Flags().StringVar(&f.timezone, "timezone", "", "IANA timezone (default: controller sysinfo timezone, else UTC)")
	if clients {
		cmd.Flags().StringVar(&f.sort, "sort", "total", "rank by download, upload, or total")
		cmd.Flags().IntVar(&f.top, "top", network.DefaultTrafficTop, "max ranked clients to return (1-100)")
	}
}

type trafficKind int

const (
	trafficKindWAN trafficKind = iota
	trafficKindClients
	trafficKindInternet
)

func runTraffic(cmd *cobra.Command, f trafficCLIFlags, kind trafficKind) error {
	schema := network.TrafficSchemaWAN
	ranked := kind == trafficKindClients || kind == trafficKindInternet
	switch kind {
	case trafficKindClients:
		schema = network.TrafficSchemaClients
	case trafficKindInternet:
		schema = network.TrafficSchemaInternet
	}
	if ranked {
		sort := strings.ToLower(strings.TrimSpace(f.sort))
		switch sort {
		case "download", "upload", "total":
		default:
			return exitf(exitcode.Usage, "invalid --sort %q (use download, upload, or total)", f.sort)
		}
		if f.top < 1 || f.top > network.MaxTrafficTop {
			return exitf(exitcode.Usage, "--top must be 1-%d", network.MaxTrafficTop)
		}
	}
	return withDiagSite(cmd, func(api *network.API, siteID, slug string) error {
		site := network.TrafficSite{ID: siteID, InternalReference: slug}
		if page, err := api.Sites(cmd.Context(), 0, 25); err == nil {
			for _, s := range page.Data {
				if s.ID == siteID || s.InternalRef == slug {
					site.Name = s.Name
					site.ID = s.ID
					if s.InternalRef != "" {
						site.InternalReference = s.InternalRef
					}
					break
				}
			}
		}
		tzName := strings.TrimSpace(f.timezone)
		tzSource := "utc"
		if tzName != "" {
			tzSource = "flag"
		} else if info, err := api.StatSysinfo(cmd.Context(), slug); err == nil {
			if z := anyString(info["timezone"]); z != "" {
				tzName = z
				tzSource = "sysinfo"
			}
		}
		loc, tzName, err := network.LoadLocationIANA(tzName)
		if err != nil {
			return exitf(exitcode.Usage, "%v", err)
		}
		observed := time.Now().In(loc)
		start, end := network.MonthToDateWindow(observed, loc)
		if strings.TrimSpace(f.start) != "" {
			start, err = network.ParseTrafficTime(f.start, loc, false)
			if err != nil {
				return exitf(exitcode.Usage, "start: %v", err)
			}
		}
		if strings.TrimSpace(f.end) != "" {
			end, err = network.ParseTrafficTime(f.end, loc, true)
			if err != nil {
				return exitf(exitcode.Usage, "end: %v", err)
			}
		}
		if err := network.ValidateTrafficRange(start, end, loc); err != nil {
			return exitf(exitcode.Usage, "%v", err)
		}
		q := network.TrafficQuery{
			Start:               start,
			End:                 end,
			Location:            loc,
			Timezone:            tzName,
			ObservedAt:          observed,
			Sort:                strings.ToLower(strings.TrimSpace(f.sort)),
			Offset:              rootOpts.offset,
			Top:                 f.top,
			All:                 rootOpts.allPages,
			ExcludeUnidentified: f.excludeUnidentified,
		}
		if info, err := api.Info(cmd.Context()); err == nil {
			q.AppVersion = info.ApplicationVersion
		}
		switch kind {
		case trafficKindClients:
			rep, err := api.ClientTraffic(cmd.Context(), site, q)
			if err != nil {
				return trafficAPIError(cmd, schema, &site, &q, err)
			}
			applyTrafficTZNotes(&rep.Limitations, tzSource)
			return printValue(cmd, rep, func() {
				printClientTrafficTable(cmd, rep)
			})
		case trafficKindInternet:
			rep, err := api.InternetTraffic(cmd.Context(), site, q)
			if err != nil {
				return trafficAPIError(cmd, schema, &site, &q, err)
			}
			applyTrafficTZNotes(&rep.Limitations, tzSource)
			return printValue(cmd, rep, func() {
				printInternetTrafficTable(cmd, rep)
			})
		default:
			rep, err := api.WANTraffic(cmd.Context(), site, q)
			if err != nil {
				return trafficAPIError(cmd, schema, &site, &q, err)
			}
			applyTrafficTZNotes(&rep.Limitations, tzSource)
			return printValue(cmd, rep, func() {
				printWANTrafficTable(cmd, rep)
			})
		}
	})
}

func applyTrafficTZNotes(lim *[]string, tzSource string) {
	if tzSource == "sysinfo" {
		*lim = append([]string{"Timezone taken from controller sysinfo."}, *lim...)
	}
	if tzSource == "utc" {
		*lim = append([]string{"Timezone omitted; using UTC."}, *lim...)
	}
}

func trafficAPIError(cmd *cobra.Command, schema string, site *network.TrafficSite, q *network.TrafficQuery, err error) error {
	status := "error"
	code := exitcode.Retryable
	lim := []string{"Missing controller report data is not replaced with zeros."}
	switch {
	case network.IsTrafficPermission(err):
		status = "denied"
		code = exitcode.Permission
	case network.IsTrafficUnsupported(err):
		status = "unsupported"
		code = exitcode.Unsupported
	default:
		var xe = mapAPIErr(err)
		if e, ok := xe.(*ExitError); ok {
			code = e.Code
		}
	}
	rep := network.NewTrafficError(schema, status, err.Error(), site, q, lim)
	_ = printValue(cmd, rep, nil)
	return exitf(code, "%s", err.Error())
}

func printWANTrafficTable(cmd *cobra.Command, rep *network.WANTrafficReport) {
	fmt.Fprintf(cmd.OutOrStdout(), "site=%s scope=%s tz=%s coverage=%s\n",
		rep.Site.Name, rep.Scope, rep.Timezone, rep.Coverage.Status)
	if rep.Totals == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "totals unavailable (not zero)")
		return
	}
	_ = output.WriteTable(cmd.OutOrStdout(), []string{"DOWNLOAD", "UPLOAD", "TOTAL", "BOUND"}, [][]string{{
		formatBytes(rep.Totals.DownloadBytes),
		formatBytes(rep.Totals.UploadBytes),
		formatBytes(rep.Totals.TotalBytes),
		emptyDash(rep.Totals.Bound),
	}})
}

func printClientTrafficTable(cmd *cobra.Command, rep *network.ClientTrafficReport) {
	fmt.Fprintf(cmd.OutOrStdout(), "site=%s scope=%s sort=%s coverage=%s\n",
		rep.Site.Name, rep.Scope, rep.Sort, rep.Coverage.Status)
	rows := make([][]string, 0, len(rep.Clients))
	for _, c := range rep.Clients {
		conn := ""
		if c.Connected != nil {
			conn = yesNo(*c.Connected)
		}
		rows = append(rows, []string{
			firstNonEmpty(c.Name, c.Hostname, c.MAC),
			c.MAC,
			formatBytes(c.DownloadBytes),
			formatBytes(c.UploadBytes),
			formatBytes(c.TotalBytes),
			conn,
		})
	}
	_ = printList(cmd, nil, []string{"NAME", "MAC", "DOWNLOAD", "UPLOAD", "TOTAL", "CONNECTED"},
		rows, rep.Ranking.Offset, rep.Ranking.Returned, rep.Ranking.RankedClients)
}

func printInternetTrafficTable(cmd *cobra.Command, rep *network.InternetTrafficReport) {
	unid := "included"
	if !rep.IncludeUnidentified {
		unid = "excluded"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "site=%s scope=%s sort=%s unidentified=%s coverage=%s\n",
		rep.Site.Name, rep.Scope, rep.Sort, unid, rep.Coverage.Status)
	if rep.Totals == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "totals unavailable (not zero)")
	} else {
		_ = output.WriteTable(cmd.OutOrStdout(), []string{"DOWNLOAD", "UPLOAD", "TOTAL"}, [][]string{{
			formatBytes(rep.Totals.DownloadBytes),
			formatBytes(rep.Totals.UploadBytes),
			formatBytes(rep.Totals.TotalBytes),
		}})
	}
	rows := make([][]string, 0, len(rep.Clients))
	for _, c := range rep.Clients {
		rows = append(rows, []string{
			firstNonEmpty(c.Name, c.Hostname, c.MAC),
			c.MAC,
			formatBytes(c.DownloadBytes),
			formatBytes(c.UploadBytes),
			formatBytes(c.TotalBytes),
		})
	}
	_ = printList(cmd, nil, []string{"NAME", "MAC", "DOWNLOAD", "UPLOAD", "TOTAL"},
		rows, rep.Ranking.Offset, rep.Ranking.Returned, rep.Ranking.RankedClients)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func formatBytes(n int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib
	)
	switch {
	case n >= tib:
		return fmt.Sprintf("%.2f TiB", float64(n)/float64(tib))
	case n >= gib:
		return fmt.Sprintf("%.2f GiB", float64(n)/float64(gib))
	case n >= mib:
		return fmt.Sprintf("%.2f MiB", float64(n)/float64(mib))
	case n >= kib:
		return fmt.Sprintf("%.2f KiB", float64(n)/float64(kib))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
