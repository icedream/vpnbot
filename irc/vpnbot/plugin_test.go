package vpnbot

import "testing"

func Test_maskedAddrRegex(t *testing.T) {
	if !maskedAddrRegex.MatchString("2BCAD547.E6EC7FF2.5266FBB4.IP") {
		t.Fail()
	}
	if !maskedAddrRegex.MatchString("Rizon-C266C4C8.lousy.speed.is.1kbps.on.this.strangled.net") {
		t.Fail()
	}
	if maskedAddrRegex.MatchString("JUST.DO.IT") {
		t.Fail()
	}
}