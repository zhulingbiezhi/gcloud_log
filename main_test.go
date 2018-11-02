package main

import "testing"

var s = `fields: <key:"exec_time" value:<number_value:0 > > fields:<key:"host" value:<string_value:"10.140.0.18" > > fields:<key:"level" value:<string_value:"info" > > fields:<key:"msg" value:<string_value:"" > > fields:<key:"request_body" value:<string_value:"" > > `

func TestScan(t *testing.T) {
	Scan(s)
}
