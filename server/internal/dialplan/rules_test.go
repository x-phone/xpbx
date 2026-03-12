package dialplan

import (
	"testing"

	"github.com/x-phone/xpbx/server/internal/database"
)

func TestRecognize_RingExtension(t *testing.T) {
	rows := []database.DialplanRule{
		{ID: 1, Context: "from-internal", Exten: "1001", Priority: 1, App: "NoOp", AppData: "Calling extension 1001"},
		{ID: 2, Context: "from-internal", Exten: "1001", Priority: 2, App: "Dial", AppData: "PJSIP/1001,20"},
		{ID: 3, Context: "from-internal", Exten: "1001", Priority: 3, App: "Hangup", AppData: ""},
	}

	groups := Recognize(rows)
	if len(groups) != 1 {
		t.Fatalf("expected 1 context group, got %d", len(groups))
	}
	if groups[0].Context != "from-internal" {
		t.Errorf("context = %q, want %q", groups[0].Context, "from-internal")
	}
	if groups[0].Label != "Internal Extensions" {
		t.Errorf("label = %q, want %q", groups[0].Label, "Internal Extensions")
	}
	if len(groups[0].Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(groups[0].Rules))
	}

	rule := groups[0].Rules[0]
	if rule.Type != RuleRingExtension {
		t.Errorf("type = %q, want %q", rule.Type, RuleRingExtension)
	}
	if rule.Target != "1001" {
		t.Errorf("target = %q, want %q", rule.Target, "1001")
	}
	if rule.Timeout != 20 {
		t.Errorf("timeout = %d, want %d", rule.Timeout, 20)
	}
	if len(rule.RowIDs) != 3 {
		t.Errorf("row IDs = %v, want 3 items", rule.RowIDs)
	}
}

func TestRecognize_RingVoicemail(t *testing.T) {
	rows := []database.DialplanRule{
		{ID: 1, Context: "from-internal", Exten: "1001", Priority: 1, App: "NoOp", AppData: "Calling extension 1001"},
		{ID: 2, Context: "from-internal", Exten: "1001", Priority: 2, App: "Dial", AppData: "PJSIP/1001,30"},
		{ID: 3, Context: "from-internal", Exten: "1001", Priority: 3, App: "VoiceMail", AppData: "1001@default,u"},
		{ID: 4, Context: "from-internal", Exten: "1001", Priority: 4, App: "Hangup", AppData: ""},
	}

	groups := Recognize(rows)
	rule := groups[0].Rules[0]
	if rule.Type != RuleRingVoicemail {
		t.Errorf("type = %q, want %q", rule.Type, RuleRingVoicemail)
	}
	if rule.Target != "1001" {
		t.Errorf("target = %q, want %q", rule.Target, "1001")
	}
	if rule.Timeout != 30 {
		t.Errorf("timeout = %d, want %d", rule.Timeout, 30)
	}
}

func TestRecognize_VoicemailOnly(t *testing.T) {
	rows := []database.DialplanRule{
		{ID: 1, Context: "from-internal", Exten: "1001", Priority: 1, App: "NoOp", AppData: "Voicemail for extension 1001"},
		{ID: 2, Context: "from-internal", Exten: "1001", Priority: 2, App: "VoiceMail", AppData: "1001@default"},
		{ID: 3, Context: "from-internal", Exten: "1001", Priority: 3, App: "Hangup", AppData: ""},
	}

	groups := Recognize(rows)
	rule := groups[0].Rules[0]
	if rule.Type != RuleVoicemailOnly {
		t.Errorf("type = %q, want %q", rule.Type, RuleVoicemailOnly)
	}
	if rule.Label != "Straight to voicemail" {
		t.Errorf("label = %q, want %q", rule.Label, "Straight to voicemail")
	}
}

func TestRecognize_RouteToTrunk(t *testing.T) {
	rows := []database.DialplanRule{
		{ID: 1, Context: "from-internal", Exten: "2001", Priority: 1, App: "NoOp", AppData: "Route 2001 via trunk my-trunk"},
		{ID: 2, Context: "from-internal", Exten: "2001", Priority: 2, App: "Dial", AppData: "PJSIP/${EXTEN}@my-trunk,30"},
		{ID: 3, Context: "from-internal", Exten: "2001", Priority: 3, App: "Hangup", AppData: ""},
	}

	groups := Recognize(rows)
	rule := groups[0].Rules[0]
	if rule.Type != RuleRouteToTrunk {
		t.Errorf("type = %q, want %q", rule.Type, RuleRouteToTrunk)
	}
	if rule.TrunkName != "my-trunk" {
		t.Errorf("trunk = %q, want %q", rule.TrunkName, "my-trunk")
	}
	if rule.Timeout != 30 {
		t.Errorf("timeout = %d, want %d", rule.Timeout, 30)
	}
}

func TestRecognize_InboundRoute(t *testing.T) {
	rows := []database.DialplanRule{
		{ID: 1, Context: "from-trunk", Exten: "_X.", Priority: 1, App: "NoOp", AppData: "Inbound call"},
		{ID: 2, Context: "from-trunk", Exten: "_X.", Priority: 2, App: "Dial", AppData: "PJSIP/${EXTEN},25"},
		{ID: 3, Context: "from-trunk", Exten: "_X.", Priority: 3, App: "Hangup", AppData: ""},
	}

	groups := Recognize(rows)
	if groups[0].Context != "from-trunk" {
		t.Errorf("context = %q, want %q", groups[0].Context, "from-trunk")
	}
	rule := groups[0].Rules[0]
	if rule.Type != RuleInboundRoute {
		t.Errorf("type = %q, want %q", rule.Type, RuleInboundRoute)
	}
	if rule.Timeout != 25 {
		t.Errorf("timeout = %d, want %d", rule.Timeout, 25)
	}
}

func TestRecognize_Custom(t *testing.T) {
	rows := []database.DialplanRule{
		{ID: 1, Context: "from-internal", Exten: "*97", Priority: 1, App: "Answer", AppData: ""},
		{ID: 2, Context: "from-internal", Exten: "*97", Priority: 2, App: "VoiceMailMain", AppData: "${CALLERID(num)}@default"},
		{ID: 3, Context: "from-internal", Exten: "*97", Priority: 3, App: "Hangup", AppData: ""},
	}

	groups := Recognize(rows)
	rule := groups[0].Rules[0]
	if rule.Type != RuleCustom {
		t.Errorf("type = %q, want %q", rule.Type, RuleCustom)
	}
	if rule.Label != "Answer -> VoiceMailMain -> Hangup" {
		t.Errorf("label = %q, want %q", rule.Label, "Answer -> VoiceMailMain -> Hangup")
	}
}

func TestRecognize_MultipleContexts(t *testing.T) {
	rows := []database.DialplanRule{
		{ID: 1, Context: "from-internal", Exten: "1001", Priority: 1, App: "NoOp", AppData: ""},
		{ID: 2, Context: "from-internal", Exten: "1001", Priority: 2, App: "Dial", AppData: "PJSIP/1001,20"},
		{ID: 3, Context: "from-internal", Exten: "1001", Priority: 3, App: "Hangup", AppData: ""},
		{ID: 4, Context: "from-trunk", Exten: "_X.", Priority: 1, App: "NoOp", AppData: ""},
		{ID: 5, Context: "from-trunk", Exten: "_X.", Priority: 2, App: "Dial", AppData: "PJSIP/${EXTEN},20"},
		{ID: 6, Context: "from-trunk", Exten: "_X.", Priority: 3, App: "Hangup", AppData: ""},
	}

	groups := Recognize(rows)
	if len(groups) != 2 {
		t.Fatalf("expected 2 context groups, got %d", len(groups))
	}
	if groups[0].Context != "from-internal" {
		t.Errorf("first context = %q, want %q", groups[0].Context, "from-internal")
	}
	if groups[1].Context != "from-trunk" {
		t.Errorf("second context = %q, want %q", groups[1].Context, "from-trunk")
	}
}

func TestRecognize_EmptyRows(t *testing.T) {
	groups := Recognize(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 20},
		{"30", 30},
		{"0", 0},
		{"abc", 20},
		{"15", 15},
	}
	for _, tt := range tests {
		got := parseTimeout(tt.input)
		if got != tt.want {
			t.Errorf("parseTimeout(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestContextLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"from-internal", "Internal Extensions"},
		{"from-trunk", "Inbound from trunk"},
		{"from-voiceworker", "Inbound from voiceworker"},
		{"custom-context", "custom-context"},
	}
	for _, tt := range tests {
		got := contextLabel(tt.input)
		if got != tt.want {
			t.Errorf("contextLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDialRegex(t *testing.T) {
	// dialDirectRe
	if m := dialDirectRe.FindStringSubmatch("PJSIP/1001,20"); m == nil {
		t.Error("dialDirectRe should match PJSIP/1001,20")
	} else if m[1] != "1001" || m[2] != "20" {
		t.Errorf("dialDirectRe got %v, want [1001, 20]", m[1:])
	}

	if m := dialDirectRe.FindStringSubmatch("PJSIP/1001"); m == nil {
		t.Error("dialDirectRe should match PJSIP/1001 (no timeout)")
	} else if m[1] != "1001" {
		t.Errorf("dialDirectRe target = %q, want %q", m[1], "1001")
	}

	// dialTrunkRe
	if m := dialTrunkRe.FindStringSubmatch("PJSIP/${EXTEN}@my-trunk,30"); m == nil {
		t.Error("dialTrunkRe should match PJSIP/${EXTEN}@my-trunk,30")
	} else if m[1] != "my-trunk" || m[2] != "30" {
		t.Errorf("dialTrunkRe got %v, want [my-trunk, 30]", m[1:])
	}

	// dialVarRe
	if m := dialVarRe.FindStringSubmatch("PJSIP/${EXTEN},25"); m == nil {
		t.Error("dialVarRe should match PJSIP/${EXTEN},25")
	} else if m[1] != "25" {
		t.Errorf("dialVarRe timeout = %q, want %q", m[1], "25")
	}
}
