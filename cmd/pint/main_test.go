package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itchyny/json2yaml"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"pint": main,
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:           "tests",
		UpdateScripts: os.Getenv("UPDATE_SNAPSHOTS") == "1",
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"http": httpServer,
			"cert": tlsCert,
		},
		Setup: func(env *testscript.Env) error {
			env.Values["mocks"] = &httpMocks{resps: map[string][]httpMock{}}
			return nil
		},
	})
}

func httpServer(ts *testscript.TestScript, _ bool, args []string) {
	mocks := ts.Value("mocks").(*httpMocks)

	if len(args) == 0 {
		ts.Fatalf("! http command requires arguments")
	}
	cmd := args[0]

	switch cmd {
	case "response":
		if len(args) < 5 {
			ts.Fatalf("! http response command requires '$NAME $PATH $CODE $BODY' args, got [%s]", strings.Join(args, " "))
		}
		name := args[1]
		path := regexp.MustCompile(args[2])
		code, err := strconv.Atoi(args[3])
		ts.Check(err)
		body := strings.Join(args[4:], " ")
		mocks.add(name, httpMock{pattern: path, handler: func(w http.ResponseWriter, r *http.Request) {
			snapshotRequest(ts, r, name, path)
			w.WriteHeader(code)
			_, err = w.Write([]byte(body))
			ts.Check(err)
		}})
	case "method":
		if len(args) < 6 {
			ts.Fatalf("! http response command requires '$NAME $METHOD $PATH $CODE $BODY' args, got [%s]", strings.Join(args, " "))
		}
		name := args[1]
		meth := args[2]
		path := regexp.MustCompile(args[3])
		code, err := strconv.Atoi(args[4])
		ts.Check(err)
		body := strings.Join(args[5:], " ")
		mocks.add(name, httpMock{pattern: path, method: meth, handler: func(w http.ResponseWriter, r *http.Request) {
			snapshotRequest(ts, r, name, path)
			w.WriteHeader(code)
			_, err := w.Write([]byte(body))
			ts.Check(err)
		}})
	// http auth-response name /200 user password 200 OK
	case "auth-response":
		if len(args) < 7 {
			ts.Fatalf("! http response command requires '$NAME $PATH $USER $PASS $CODE $BODY' args, got [%s]", strings.Join(args, " "))
		}
		name := args[1]
		path := regexp.MustCompile(args[2])
		user := args[3]
		pass := args[4]
		code, err := strconv.Atoi(args[5])
		ts.Check(err)
		body := strings.Join(args[6:], " ")
		mocks.add(name, httpMock{pattern: path, handler: func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if ok && username == user && password == pass {
				snapshotRequest(ts, r, name, path)
				w.WriteHeader(code)
				_, err := w.Write([]byte(body))
				ts.Check(err)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		}})
	// http response name /200 200 OK
	case "slow-response":
		if len(args) < 6 {
			ts.Fatalf("! http response command requires '$NAME $PATH $DELAY $CODE $BODY' args, got [%s]", strings.Join(args, " "))
		}
		name := args[1]
		path := regexp.MustCompile(args[2])
		delay, err := time.ParseDuration(args[3])
		ts.Check(err)
		code, err := strconv.Atoi(args[4])
		ts.Check(err)
		body := strings.Join(args[5:], " ")
		mocks.add(name, httpMock{pattern: path, handler: func(w http.ResponseWriter, r *http.Request) {
			snapshotRequest(ts, r, name, path)
			time.Sleep(delay)
			w.WriteHeader(code)
			_, err := w.Write([]byte(body))
			ts.Check(err)
		}})
	// http redirect name /foo/src /dst
	case "redirect":
		if len(args) != 4 {
			ts.Fatalf("! http redirect command requires '$NAME $SRCPATH $DSTPATH' args, got [%s]", strings.Join(args, " "))
		}
		name := args[1]
		srcpath := regexp.MustCompile(args[2])
		dstpath := args[3]
		mocks.add(name, httpMock{pattern: srcpath, handler: func(w http.ResponseWriter, r *http.Request) {
			snapshotRequest(ts, r, name, srcpath)
			w.Header().Set("Location", dstpath)
			w.WriteHeader(http.StatusFound)
		}})
	// http start name 127.0.0.1:7088 [cert.pem cert.key]
	case "start":
		if len(args) < 3 {
			ts.Fatalf("! http start command requires '$NAME $LISTEN [$TLS_CERT $TLS_KEY]' args, got [%s]", strings.Join(args, " "))
		}
		name := args[1]
		listen := args[2]
		var isTLS bool
		var tlsCert, tlsKey string
		if len(args) == 5 {
			isTLS = true
			tlsCert = args[3]
			tlsKey = args[4]
		}

		var mtx sync.Mutex
		mux := http.NewServeMux()
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var done bool
			for n, mockList := range mocks.responses() {
				if n == name {
					for _, mock := range mockList {
						if mock.method != "" && mock.method != r.Method {
							continue
						}
						if !mock.pattern.MatchString(r.URL.Path) {
							continue
						}
						mtx.Lock()
						mock.handler(w, r)
						done = true
						mtx.Unlock()
						break
					}
					break
				}
			}
			if !done {
				w.WriteHeader(http.StatusNotFound)
			}
		}))

		listener, err := net.Listen("tcp", listen)
		ts.Check(err)
		server := &http.Server{Addr: listen, Handler: mux}
		go func() {
			var serveErr error
			if isTLS {
				serveErr = server.ServeTLS(listener, tlsCert, tlsKey)
			} else {
				serveErr = server.Serve(listener)
			}
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				fmt.Printf("http server failed to start: %s\n", serveErr)
				ts.Fatalf("http server failed to start: %s", serveErr)
			}
		}()

		ts.Defer(func() {
			ts.Logf("http server %s shutting down", name)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		})
	default:
		ts.Fatalf("! unknown http command: %v", args)
	}
}

type httpMock struct {
	pattern *regexp.Regexp
	handler func(http.ResponseWriter, *http.Request)
	method  string
}

type httpMocks struct {
	resps map[string][]httpMock
	mtx   sync.Mutex
}

func (m *httpMocks) add(name string, mock httpMock) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if _, ok := m.resps[name]; !ok {
		m.resps[name] = []httpMock{}
	}
	m.resps[name] = append(m.resps[name], mock)
}

func (m *httpMocks) responses() map[string][]httpMock {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.resps
}

func tlsCert(ts *testscript.TestScript, _ bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("! cert command requires '$DIRNAME $NAME' args, got [%s]", strings.Join(args, " "))
	}
	dirname := args[0]
	name := args[1]

	ts.Logf("test-script cert command: %s", strings.Join(args, " "))

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		ts.Fatalf("failed to generate CA private key: %s", err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		ts.Fatalf("failed to generate CA cert: %s", err)
	}

	writeCert(ts, dirname, name+"-ca.pem", &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	writeCert(ts, dirname, name+"-ca.key", &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization:  []string{""},
			Country:       []string{""},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		DNSNames:     []string{name},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 1),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		ts.Fatalf("failed to generate cert private key: %s", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		ts.Fatalf("failed to generate cert: %s", err)
	}

	writeCert(ts, dirname, name+".pem", &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	writeCert(ts, dirname, name+".key", &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})
}

func writeCert(ts *testscript.TestScript, dirname, filename string, block *pem.Block) {
	fullpath := path.Join(dirname, filename)

	f, err := os.Create(fullpath)
	if err != nil {
		ts.Fatalf("failed to write %s: %s", fullpath, err)
	}

	if err = pem.Encode(f, block); err != nil {
		ts.Fatalf("failed to encode %s: %s", fullpath, err)
	}

	if err = f.Close(); err != nil {
		ts.Fatalf("failed to close %s: %s", fullpath, err)
	}

	ts.Logf("Wrote PEM file to %s", filename)
}

func snapshotRequest(ts *testscript.TestScript, r *http.Request, name string, path *regexp.Regexp) {
	payload, err := io.ReadAll(r.Body)
	ts.Check(err)
	r.Body.Close()

	var buf strings.Builder
	buf.WriteString(r.Method)
	buf.WriteRune(' ')
	buf.WriteString(path.String())
	buf.WriteRune('\n')

	hKeys := make([]string, 0, len(r.Header))
	for k := range r.Header {
		if k == "Content-Length" || k == "User-Agent" {
			continue
		}
		hKeys = append(hKeys, k)
	}
	slices.Sort(hKeys)
	for _, k := range hKeys {
		buf.WriteString("  ")
		buf.WriteString(k)
		buf.WriteRune(':')
		buf.WriteRune(' ')
		buf.WriteString(strings.Join(r.Header.Values(k), ";"))
		buf.WriteRune('\n')
	}

	if len(payload) > 0 {
		payload = sanitizePayload(payload)
		buf.WriteString("--- BODY ---\n")
		buf.Write(payload)
		if !bytes.HasSuffix(payload, []byte("\n")) {
			buf.WriteRune('\n')
		}
		buf.WriteString("--- END ---\n")
	}
	buf.WriteRune('\n')

	f, err := os.OpenFile(ts.MkAbs(name+".got"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	ts.Check(err)
	_, err = f.WriteString(buf.String())
	ts.Check(err)
	ts.Check(f.Close())
}

func sanitizePayload(payload []byte) []byte {
	bbDurationRe := regexp.MustCompile(`\{"value":([0-9]+),"title":"Checks duration","type":"DURATION"\}`)
	payload = bbDurationRe.ReplaceAll(payload, []byte(`{"value":0,"title":"Checks duration","type":"DURATION"}`))

	ghDurationRe := regexp.MustCompile(`\| Checks duration \| ([0-9]+[a-z]+) \|`)
	payload = ghDurationRe.ReplaceAll(payload, []byte(`| Checks duration | 0 |`))

	commitIDRe := regexp.MustCompile(`"commit_id":"([0-9a-zA-Z]{40})"`)
	payload = commitIDRe.ReplaceAll(payload, []byte(`"commit_id":"<COMMIT ID>"`))

	var output bytes.Buffer
	if err := json2yaml.Convert(&output, bytes.NewReader(payload)); err != nil {
		return payload
	}

	return output.Bytes()
}
