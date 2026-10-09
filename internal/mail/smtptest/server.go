package smtptest

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type Mode int

const (
	Plain Mode = iota
	ImplicitTLS
	StartTLS
)

type Options struct {
	Mode           Mode
	Username       string
	Password       string
	AuthMechanisms string
	Stall          bool
	RejectRcpt     bool
}

type Received struct {
	From     string
	To       []string
	Data     string
	AuthUser string
}

type Server struct {
	opts     Options
	listener net.Listener
	cert     tls.Certificate
	roots    *x509.CertPool

	mu       sync.Mutex
	messages []Received
	conns    []net.Conn
}

func Start(t testing.TB, opts Options) *Server {
	t.Helper()
	cert, roots := selfSigned(t)
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if opts.AuthMechanisms == "" {
		opts.AuthMechanisms = "PLAIN LOGIN"
	}
	s := &Server{opts: opts, listener: listener, cert: cert, roots: roots}
	go s.accept()
	t.Cleanup(s.Close)
	return s
}

func (s *Server) Close() {
	if err := s.listener.Close(); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, conn := range s.conns {
		if err := conn.Close(); err != nil {
			continue
		}
	}
}

func (s *Server) Host() string { return "127.0.0.1" }

func (s *Server) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *Server) Addr() string {
	return net.JoinHostPort(s.Host(), strconv.Itoa(s.Port()))
}

func (s *Server) Roots() *x509.CertPool { return s.roots }

func (s *Server) TLSConfig() *tls.Config {
	return &tls.Config{RootCAs: s.roots, MinVersion: tls.VersionTLS12}
}

func (s *Server) Messages() []Received {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Received(nil), s.messages...)
}

func (s *Server) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, conn)
		s.mu.Unlock()
		go s.serve(conn)
	}
}

func selfSigned(t testing.TB) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
		IsCA:         true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}

type session struct {
	server   *Server
	conn     net.Conn
	reader   *bufio.Reader
	tls      bool
	user     string
	from     string
	to       []string
	loggedIn bool
}

func (s *Server) serve(conn net.Conn) {
	sess := &session{server: s, conn: conn}
	if s.opts.Mode == ImplicitTLS {
		conn = tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS12})
		sess.conn = conn
		sess.tls = true
	}
	sess.reader = bufio.NewReader(conn)
	if s.opts.Stall {
		time.Sleep(time.Minute)
		return
	}
	sess.reply("220 smtptest ready")
	sess.loop()
}

func (c *session) reply(line string) {
	if _, err := fmt.Fprintf(c.conn, "%s\r\n", line); err != nil {
		return
	}
}

func (c *session) readLine() (string, bool) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", false
	}
	return strings.TrimRight(line, "\r\n"), true
}

func (c *session) loop() {
	for {
		line, ok := c.readLine()
		if !ok {
			return
		}
		verb, rest, _ := strings.Cut(line, " ")
		if !c.dispatch(strings.ToUpper(verb), rest) {
			return
		}
	}
}

func (c *session) dispatch(verb, rest string) bool {
	switch verb {
	case "EHLO", "HELO":
		c.hello()
	case "STARTTLS":
		return c.upgrade()
	case "AUTH":
		c.authenticate(rest)
	case "MAIL":
		c.mailFrom(rest)
	case "RCPT":
		c.rcptTo(rest)
	case "DATA":
		c.data()
	case "RSET", "NOOP":
		c.reply("250 ok")
	case "QUIT":
		c.reply("221 bye")
		return false
	default:
		c.reply("502 command not implemented")
	}
	return true
}

func (c *session) hello() {
	lines := []string{"smtptest"}
	if c.server.opts.Mode == StartTLS && !c.tls {
		lines = append(lines, "STARTTLS")
	}
	if c.server.opts.Username != "" {
		lines = append(lines, "AUTH "+c.server.opts.AuthMechanisms)
	}
	for i, line := range lines {
		separator := "-"
		if i == len(lines)-1 {
			separator = " "
		}
		c.reply("250" + separator + line)
	}
}

func (c *session) upgrade() bool {
	if c.server.opts.Mode != StartTLS || c.tls {
		c.reply("502 command not implemented")
		return true
	}
	c.reply("220 ready to start TLS")
	c.conn = tls.Server(c.conn, &tls.Config{Certificates: []tls.Certificate{c.server.cert}, MinVersion: tls.VersionTLS12})
	c.reader = bufio.NewReader(c.conn)
	c.tls = true
	return true
}

func (c *session) authenticate(rest string) {
	mechanism, initial, _ := strings.Cut(rest, " ")
	var user, pass string
	switch strings.ToUpper(mechanism) {
	case "PLAIN":
		if initial == "" {
			c.reply("334 ")
			line, ok := c.readLine()
			if !ok {
				return
			}
			initial = line
		}
		parts := strings.Split(decode(initial), "\x00")
		if len(parts) == 3 {
			user, pass = parts[1], parts[2]
		}
	case "LOGIN":
		var ok bool
		if user, pass, ok = c.loginExchange(); !ok {
			return
		}
	default:
		c.reply("504 unsupported mechanism")
		return
	}
	if user == c.server.opts.Username && pass == c.server.opts.Password {
		c.loggedIn, c.user = true, user
		c.reply("235 2.7.0 authenticated")
		return
	}
	c.reply("535 5.7.8 authentication credentials invalid")
}

func (c *session) loginExchange() (string, string, bool) {
	c.reply("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
	userLine, ok := c.readLine()
	if !ok {
		return "", "", false
	}
	c.reply("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
	passLine, ok := c.readLine()
	if !ok {
		return "", "", false
	}
	return decode(userLine), decode(passLine), true
}

func decode(value string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return string(raw)
}

func (c *session) needsAuth() bool {
	return c.server.opts.Username != "" && !c.loggedIn
}

func (c *session) mailFrom(rest string) {
	if c.needsAuth() {
		c.reply("530 5.7.0 authentication required")
		return
	}
	c.from = address(rest)
	c.to = nil
	c.reply("250 ok")
}

func (c *session) rcptTo(rest string) {
	if c.server.opts.RejectRcpt {
		c.reply("550 5.1.1 mailbox unavailable")
		return
	}
	c.to = append(c.to, address(rest))
	c.reply("250 ok")
}

func address(rest string) string {
	_, value, _ := strings.Cut(rest, ":")
	value = strings.TrimSpace(value)
	start, end := strings.Index(value, "<"), strings.Index(value, ">")
	if start >= 0 && end > start {
		return value[start+1 : end]
	}
	return value
}

func (c *session) data() {
	if len(c.to) == 0 {
		c.reply("503 need RCPT first")
		return
	}
	c.reply("354 end with <CRLF>.<CRLF>")
	var body strings.Builder
	for {
		line, ok := c.readLine()
		if !ok {
			return
		}
		if line == "." {
			break
		}
		body.WriteString(strings.TrimPrefix(line, ".") + "\n")
	}
	c.server.mu.Lock()
	c.server.messages = append(c.server.messages, Received{From: c.from, To: c.to, Data: body.String(), AuthUser: c.user})
	c.server.mu.Unlock()
	c.reply("250 queued")
}
