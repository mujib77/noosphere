package main

import "testing"

func TestReadLoadAveragesParsesProcData(t *testing.T) {
	load1, load5, load15, err := parseLoadAverages("1.25 0.80 0.42 2/123 456")

	if err != nil {
		t.Fatal(err)
	}

	if load1 != 1.25 || load5 != 0.80 || load15 != 0.42 {
		t.Fatalf(
			"unexpected load averages: %f %f %f",
			load1,
			load5,
			load15,
		)
	}
}

func TestEBPFReadyRequiresBTFAndBPFFS(t *testing.T) {
	if isEBPFReady(false, false) {
		t.Fatal("expected eBPF readiness to be false")
	}

	if isEBPFReady(true, false) {
		t.Fatal("expected eBPF readiness to require bpffs")
	}

	if isEBPFReady(false, true) {
		t.Fatal("expected eBPF readiness to require BTF")
	}

	if !isEBPFReady(true, true) {
		t.Fatal("expected eBPF readiness when BTF and bpffs exist")
	}
}
