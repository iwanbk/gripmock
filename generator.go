package main

import (
	"io"
	"os"
	"text/template"
)

type generatorParam struct {
	Proto
	GrpcPort  string
	AdminPort string
	PbPath    string
}

type Options struct {
	writer    io.Writer
	grpcPort  string
	adminPort string
	pbPath    string
}

func GenerateServerFromProto(proto Proto, opt *Options) error {
	param := generatorParam{
		Proto:     proto,
		GrpcPort:  opt.grpcPort,
		AdminPort: opt.adminPort,
		PbPath:    opt.pbPath,
	}

	if opt == nil {
		opt = &Options{}
	}

	if opt.writer == nil {
		opt.writer = os.Stdout
	}

	tmpl := template.New("server.tmpl")
	tmpl, err := tmpl.Parse(SERVER_TEMPLATE)
	if err != nil {
		return err
	}

	return tmpl.Execute(opt.writer, param)
}

const SERVER_TEMPLATE = `
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	TCP_PORT  = ":{{.GrpcPort}}"
	HTTP_PORT = ":{{.AdminPort}}"
)

{{ template "services" .Services }}

func main() {
	lis, err := net.Listen("tcp", TCP_PORT)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	{{ template "register_services" .Services }}

	reflection.Register(s)
	log.Println("Serving gRPC on tcp://localost" + TCP_PORT)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

{{ template "find_stub" }}

{{ define "services" }}
{{range . }}
type {{.Name}} struct{}

{{ template "methods" .}}

{{end}}
{{ end }}

{{ define "methods" }}
{{ $serviceName := .Name }}
{{ range .Methods}}
func (s *{{$serviceName}}) {{.Name}}(ctx context.Context, in *{{.Input}}) (*{{.Output}},error){
	out := &{{.Output}}{}
	err := findStub("{{$serviceName}}", ".Name", in, out)
	return out, err
}
{{end}}
{{end}}

{{ define "register_services" }}
{{ range .}}
	Register{{.Name}}Server(s, &{{.Name}}{})
{{ end }}
{{ end }}

{{ define "find_stub" }}
type payload struct {
	Service string      ` + "`json:\"service\"`" + `
	Method  string      ` + "`json:\"method\"`" + `
	Data    interface{} ` + "`json:\"data\"`" + `
}

type response struct {
	Data  interface{} ` + "`json:\"data\"`" + `
	Error string      ` + "`json:\"error\"`" + `
}

func findStub(service, method string, in, out interface{}) error {
	url := fmt.Sprintf("http://localhost%s/find", HTTP_PORT)
	pyl := payload{
		Service: service,
		Method:  method,
		Data:    in,
	}
	byt, err := json.Marshal(pyl)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(byt)
	resp, err := http.DefaultClient.Post(url, "application/json", reader)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf(string(body))
	}

	respRPC := new(response)
	err = json.NewDecoder(resp.Body).Decode(respRPC)
	if err != nil {
		return err
	}

	if respRPC.Error != "" {
		return fmt.Errorf(respRPC.Error)
	}

	return mapstructure.Decode(respRPC.Data, out)
}
{{ end }}`
