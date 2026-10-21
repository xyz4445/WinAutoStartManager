//go:build windows

package gui

import "testing"

func TestWindowMetricsKeepLogicalSizeAt96DPI(t *testing.T) {
	got := windowMetricsForDPI(96, 1920, 1040)
	want := windowMetrics{Width: 1000, Height: 740, MinWidth: 860, MinHeight: 560}
	if got != want {
		t.Fatalf("metrics at 96 DPI = %+v, want %+v", got, want)
	}
}

func TestWindowMetricsScaleLogicalSizeWithDPI(t *testing.T) {
	got := windowMetricsForDPI(192, 3840, 2080)
	want := windowMetrics{Width: 2000, Height: 1480, MinWidth: 1720, MinHeight: 1120}
	if got != want {
		t.Fatalf("metrics at 192 DPI = %+v, want %+v", got, want)
	}
}

func TestWindowMetricsFitInsideWorkArea(t *testing.T) {
	got := windowMetricsForDPI(144, 1920, 1040)
	want := windowMetrics{Width: 1500, Height: 936, MinWidth: 1290, MinHeight: 840}
	if got != want {
		t.Fatalf("metrics at 144 DPI = %+v, want %+v", got, want)
	}
}

func TestWindowMetricsClampMinSizeToWindow(t *testing.T) {
	got := windowMetricsForDPI(300, 1920, 1040)
	want := windowMetrics{Width: 1728, Height: 936, MinWidth: 1728, MinHeight: 936}
	if got != want {
		t.Fatalf("metrics at 300 DPI = %+v, want %+v", got, want)
	}
}

func TestWindowMetricsWithoutWorkAreaOnlyScale(t *testing.T) {
	got := windowMetricsForDPI(120, 0, 0)
	want := windowMetrics{Width: 1250, Height: 925, MinWidth: 1075, MinHeight: 700}
	if got != want {
		t.Fatalf("metrics without work area = %+v, want %+v", got, want)
	}
}

func TestWindowMetricsTreatInvalidDPIAs96(t *testing.T) {
	got := windowMetricsForDPI(0, 1920, 1040)
	want := windowMetrics{Width: 1000, Height: 740, MinWidth: 860, MinHeight: 560}
	if got != want {
		t.Fatalf("metrics with invalid DPI = %+v, want %+v", got, want)
	}
}

func TestInitialWindowOptionsUseScaledSize(t *testing.T) {
	opts := initialWindowOptions(192, 3840, 2080)
	if opts.Width != 2000 || opts.Height != 1480 {
		t.Fatalf("window size = %dx%d, want 2000x1480", opts.Width, opts.Height)
	}
	if !opts.Center {
		t.Fatal("window should be centered")
	}
}

func TestProcessBecomesPerMonitorDPIAware(t *testing.T) {
	if err := enablePerMonitorDPI(); err != nil {
		t.Fatalf("enablePerMonitorDPI: %v", err)
	}
	awareness, err := processDPIAwareness()
	if err != nil {
		t.Fatalf("processDPIAwareness: %v", err)
	}
	if awareness != processPerMonitorDPIAware {
		t.Fatalf("DPI awareness = %d, want %d", awareness, processPerMonitorDPIAware)
	}
}
