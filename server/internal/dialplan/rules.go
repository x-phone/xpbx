package dialplan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/x-phone/xpbx/server/internal/database"
)

// RuleType identifies the kind of routing rule.
type RuleType string

const (
	RuleRingExtension  RuleType = "ring_extension"
	RuleRingVoicemail  RuleType = "ring_voicemail"
	RuleVoicemailOnly  RuleType = "voicemail_only"
	RuleRouteToTrunk   RuleType = "route_to_trunk"
	RuleInboundRoute   RuleType = "inbound_route"
	RuleCustom         RuleType = "custom"
)

// RoutingRule is a friendly representation of a group of dialplan rows.
type RoutingRule struct {
	Type    RuleType
	Context string
	Pattern string // Extension pattern (e.g. "1001", "_2XXX", "_X.")
	Label   string // Human-readable description

	// Ring Extension fields
	Target  string // e.g. "PJSIP/1001"
	Timeout int    // Dial timeout in seconds

	// Route to Trunk fields
	TrunkName string // e.g. "my-trunk"

	// Raw rows backing this rule
	RowIDs []int64
	Rows   []database.DialplanRule
}

// ContextGroup groups routing rules by their dialplan context.
type ContextGroup struct {
	Context     string
	Label       string // Friendly label for the context
	Rules       []RoutingRule
}

// Regex to parse Dial app data
var (
	dialDirectRe = regexp.MustCompile(`^PJSIP/([^,]+?)(?:,(\d+))?$`)
	dialTrunkRe  = regexp.MustCompile(`^PJSIP/\$\{EXTEN\}@([^,]+?)(?:,(\d+))?$`)
	dialVarRe    = regexp.MustCompile(`^PJSIP/\$\{EXTEN\}(?:,(\d+))?$`)
)

// Recognize takes raw dialplan rows and groups them into friendly routing rules.
func Recognize(rows []database.DialplanRule) []ContextGroup {
	// Group rows by (context, exten)
	type key struct{ context, exten string }
	grouped := map[key][]database.DialplanRule{}
	order := []key{}

	for _, r := range rows {
		k := key{r.Context, r.Exten}
		if _, exists := grouped[k]; !exists {
			order = append(order, k)
		}
		grouped[k] = append(grouped[k], r)
	}

	// Recognize each group into a routing rule
	contextRules := map[string][]RoutingRule{}
	contextOrder := []string{}

	for _, k := range order {
		group := grouped[k]
		rule := recognizeGroup(k.context, k.exten, group)

		if _, exists := contextRules[k.context]; !exists {
			contextOrder = append(contextOrder, k.context)
		}
		contextRules[k.context] = append(contextRules[k.context], rule)
	}

	// Build context groups
	var result []ContextGroup
	for _, ctx := range contextOrder {
		result = append(result, ContextGroup{
			Context: ctx,
			Label:   contextLabel(ctx),
			Rules:   contextRules[ctx],
		})
	}
	return result
}

// recognizeGroup tries to match a set of rows for a single (context, exten) to a known pattern.
func recognizeGroup(context, exten string, rows []database.DialplanRule) RoutingRule {
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}

	// Find key rows
	var dialRow *database.DialplanRule
	var vmRow *database.DialplanRule
	for i := range rows {
		switch {
		case strings.EqualFold(rows[i].App, "Dial"):
			dialRow = &rows[i]
		case strings.EqualFold(rows[i].App, "VoiceMail"):
			vmRow = &rows[i]
		}
	}

	// Voicemail-only pattern: no Dial, just VoiceMail
	if dialRow == nil && vmRow != nil {
		return RoutingRule{
			Type:    RuleVoicemailOnly,
			Context: context,
			Pattern: exten,
			Label:   "Straight to voicemail",
			RowIDs:  ids,
			Rows:    rows,
		}
	}

	if dialRow == nil {
		return customRule(context, exten, rows, ids)
	}

	// Pattern: Route to Trunk — Dial(PJSIP/${EXTEN}@my-trunk,30)
	// Checked before dialDirectRe because dialDirectRe's [^,]+? would also match ${EXTEN}@trunk.
	if m := dialTrunkRe.FindStringSubmatch(dialRow.AppData); m != nil {
		timeout := parseTimeout(m[2])
		return RoutingRule{
			Type:      RuleRouteToTrunk,
			Context:   context,
			Pattern:   exten,
			TrunkName: m[1],
			Timeout:   timeout,
			Label:     fmt.Sprintf("Route via %s (%ds)", m[1], timeout),
			RowIDs:    ids,
			Rows:      rows,
		}
	}

	// Pattern: Inbound Route — Dial(PJSIP/${EXTEN},30)
	if m := dialVarRe.FindStringSubmatch(dialRow.AppData); m != nil {
		timeout := parseTimeout(m[1])
		return RoutingRule{
			Type:    RuleInboundRoute,
			Context: context,
			Pattern: exten,
			Timeout: timeout,
			Label:   fmt.Sprintf("Ring matching extension (%ds)", timeout),
			RowIDs:  ids,
			Rows:    rows,
		}
	}

	// Pattern: Ring Extension — Dial(PJSIP/1001,20)
	// Checked after trunk/inbound patterns because dialDirectRe's [^,]+? is broad.
	if m := dialDirectRe.FindStringSubmatch(dialRow.AppData); m != nil {
		timeout := parseTimeout(m[2])
		// Check if followed by VoiceMail
		if vmRow != nil {
			return RoutingRule{
				Type:    RuleRingVoicemail,
				Context: context,
				Pattern: exten,
				Target:  m[1],
				Timeout: timeout,
				Label:   fmt.Sprintf("Ring %s (%ds), then voicemail", m[1], timeout),
				RowIDs:  ids,
				Rows:    rows,
			}
		}
		return RoutingRule{
			Type:    RuleRingExtension,
			Context: context,
			Pattern: exten,
			Target:  m[1],
			Timeout: timeout,
			Label:   fmt.Sprintf("Ring %s (%ds)", m[1], timeout),
			RowIDs:  ids,
			Rows:    rows,
		}
	}

	return customRule(context, exten, rows, ids)
}

func customRule(context, exten string, rows []database.DialplanRule, ids []int64) RoutingRule {
	// Build a summary from the apps
	apps := make([]string, len(rows))
	for i, r := range rows {
		apps[i] = r.App
	}
	return RoutingRule{
		Type:    RuleCustom,
		Context: context,
		Pattern: exten,
		Label:   strings.Join(apps, " -> "),
		RowIDs:  ids,
		Rows:    rows,
	}
}

func parseTimeout(s string) int {
	if s == "" {
		return 20 // Asterisk default
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 20
	}
	return n
}

func contextLabel(ctx string) string {
	switch {
	case ctx == "from-internal":
		return "Internal Extensions"
	case strings.HasPrefix(ctx, "from-"):
		name := strings.TrimPrefix(ctx, "from-")
		return "Inbound from " + name
	default:
		return ctx
	}
}
