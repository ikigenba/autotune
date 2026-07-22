package cli

import (
	"reflect"
	"testing"
	"time"
)

// R-RR0J-54PO
func TestParseAcceptsValueFormsRepeatableConfigAndDefaults(t *testing.T) {
	defaults, err := Parse([]string{"work"})
	if err != nil {
		t.Fatalf("Parse defaults: %v", err)
	}
	if defaults.Folder != "work" || defaults.Repeat != 3 || defaults.Parallel != 4 || defaults.Worst != 5 || defaults.MaxRetries != 3 || defaults.MaxIterations != 0 || defaults.MaxTime != 0 || defaults.MaxSpend != 0 {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}

	got, err := Parse([]string{
		"-c", "model.name=first", "-c=model.name=second",
		"--repeat", "6", "--parallel=7", "--worst", "8",
		"--max-retries=9", "--max-iterations", "10",
		"--max-time=2h30m", "--max-spend", "12.5", "work",
	})
	if err != nil {
		t.Fatalf("Parse explicit values: %v", err)
	}
	if !reflect.DeepEqual(got.Config, []string{"model.name=first", "model.name=second"}) {
		t.Fatalf("Config = %#v", got.Config)
	}
	if got.Repeat != 6 || got.Parallel != 7 || got.Worst != 8 || got.MaxRetries != 9 || got.MaxIterations != 10 || got.MaxTime != 2*time.Hour+30*time.Minute || got.MaxSpend != 12.5 {
		t.Fatalf("unexpected explicit values: %+v", got)
	}

	forms := []struct {
		name  string
		value string
		check func(Options) bool
	}{
		{"--repeat", "11", func(o Options) bool { return o.Repeat == 11 }},
		{"--parallel", "12", func(o Options) bool { return o.Parallel == 12 }},
		{"--worst", "13", func(o Options) bool { return o.Worst == 13 }},
		{"--max-retries", "14", func(o Options) bool { return o.MaxRetries == 14 }},
		{"--max-iterations", "15", func(o Options) bool { return o.MaxIterations == 15 }},
		{"--max-time", "16m", func(o Options) bool { return o.MaxTime == 16*time.Minute }},
		{"--max-spend", "17.5", func(o Options) bool { return o.MaxSpend == 17.5 }},
	}
	for _, form := range forms {
		for _, args := range [][]string{{form.name, form.value, "work"}, {form.name + "=" + form.value, "work"}} {
			opts, err := Parse(args)
			if err != nil || !form.check(opts) {
				t.Errorf("Parse(%q) = %+v, %v", args, opts, err)
			}
		}
	}
}

// R-RS8F-IWGD
func TestParseClassifiesUsageAndTerminalFlags(t *testing.T) {
	bad := [][]string{
		{"--unknown", "work"},
		{},
		{"one", "two"},
		{"--repeat", "nope", "work"},
		{"--max-time=-1s", "work"},
		{"--max-spend=NaN", "work"},
	}
	for _, args := range bad {
		_, err := Parse(args)
		if err == nil || !IsUsageError(err) {
			t.Errorf("Parse(%q) error = %v, want usage error with exit %d", args, err, Usage.ExitCode())
		}
	}

	help, err := Parse([]string{"--help"})
	if err != nil || !help.Help {
		t.Fatalf("Parse help = %+v, %v", help, err)
	}
	version, err := Parse([]string{"--version"})
	if err != nil || !version.Version {
		t.Fatalf("Parse version = %+v, %v", version, err)
	}
	if Done.ExitCode() != 0 || Usage.ExitCode() != 2 {
		t.Fatalf("terminal exit codes: Done=%d Usage=%d", Done.ExitCode(), Usage.ExitCode())
	}
}

// R-RTGB-WO72
func TestStopReasonExitCode(t *testing.T) {
	want := map[StopReason]int{Done: 0, Failed: 1, Usage: 2, RailCrossed: 3, Interrupted: 130}
	for reason, code := range want {
		if got := reason.ExitCode(); got != code {
			t.Errorf("StopReason(%d).ExitCode() = %d, want %d", reason, got, code)
		}
	}
}
