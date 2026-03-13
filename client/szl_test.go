package client

import (
	"testing"

	"github.com/otfabric/s7comm/model"
)

func TestPopulateInfoFromRaw(t *testing.T) {
	info := &model.DeviceInfo{}
	raw := []byte("CPU 1516-3 PN/DP 6ES7516-3AN01-0AB0 FW V2.9.1 SERIAL ABCD12345678")
	populateInfoFromRaw(info, raw)
	setIdentifyDefaults(info)

	if info.OrderNumber == "N/A" {
		t.Fatal("expected order number extracted")
	}
	if info.CPUType == "N/A" {
		t.Fatal("expected cpu type extracted")
	}
	if info.FWVersion == "N/A" {
		t.Fatal("expected firmware extracted")
	}
	if info.SerialNumber == "N/A" {
		t.Fatal("expected serial extracted")
	}
}

func TestSetIdentifyDefaults(t *testing.T) {
	info := &model.DeviceInfo{}
	setIdentifyDefaults(info)
	if info.OrderNumber != "N/A" || info.CPUType != "N/A" || info.FWVersion != "N/A" {
		t.Fatal("expected N/A defaults")
	}
}
