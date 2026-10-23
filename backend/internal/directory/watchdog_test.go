package directory

import (
	"errors"
	"net"
	"testing"
	"time"

	ber "github.com/go-asn1-ber/asn1-ber"
)

// stallingServer accepts StartTLS and then never answers the TLS handshake.
func stallingServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		request, err := ber.ReadPacket(conn)
		if err != nil || len(request.Children) == 0 {
			return
		}
		response := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Response")
		response.AppendChild(request.Children[0])
		extended := ber.Encode(ber.ClassApplication, ber.TypeConstructed, 24, nil, "Extended Response")
		extended.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, 0, "resultCode"))
		extended.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "matchedDN"))
		extended.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "diagnosticMessage"))
		response.AppendChild(extended)
		if _, err := conn.Write(response.Bytes()); err != nil {
			return
		}
		// Never send a ServerHello; wait until the client gives up.
		_, _ = conn.Read(make([]byte, 1024))
		time.Sleep(time.Minute)
	}()
	return "ldap://" + ln.Addr().String()
}

func TestStalledStartTLSDoesNotHangTheSignIn(t *testing.T) {
	previous := sessionLimit
	sessionLimit = func(Config) time.Duration { return 300 * time.Millisecond }
	t.Cleanup(func() { sessionLimit = previous })

	c := Config{URLs: []string{stallingServer(t)}, StartTLS: true, Timeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() {
		_, err := open(c, Rules{}, &recorder{})
		done <- err
	}()
	select {
	case err := <-done:
		var se *stepError
		if !errors.As(err, &se) || se.step != stepTLS {
			t.Fatalf("stalled handshake: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the stalled handshake was not cut off")
	}
}
