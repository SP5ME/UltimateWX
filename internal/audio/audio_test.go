package audio

import (
	"reflect"
	"testing"
)

func TestParseDevicesPrefersAliasesAndKeepsDescriptions(t *testing.T) {
	got := parseDevices("default\n  System default device\nhw:CARD=USB,DEV=0\n  USB Audio\nplughw:CARD=USB,DEV=0\n  USB Audio plug-in\n")
	want := []Device{{Name: "default", Description: "System default device"}, {Name: "plughw:CARD=USB,DEV=0", Description: "USB Audio plug-in"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDevices() = %#v, want %#v", got, want)
	}
}

func TestParseDevicesFallsBackToHardwareWhenNoAliasesExist(t *testing.T) {
	got := parseDevices("null\n  Discard all samples\nhw:CARD=USB,DEV=0\n  USB Audio\nhw:CARD=USB,DEV=0\n  Duplicate device\n")
	want := []Device{{Name: "hw:CARD=USB,DEV=0", Description: "USB Audio"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDevices() = %#v, want %#v", got, want)
	}
}
