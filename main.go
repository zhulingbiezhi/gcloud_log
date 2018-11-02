package main

import (
	"bufio"
	"bytes"
	logv2 "cloud.google.com/go/logging/apiv2"
	"encoding/json"
	"fmt"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	loggingpb "google.golang.org/genproto/googleapis/logging/v2"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)


var refreshToken string
var accessToken  string

type auth struct {
}

func (*auth) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
		Expiry:       time.Now().Add(time.Second * 3600),
	}, nil
}

func main() {
	args := os.Args[1:]
	var execFn = func(string, string) {}
	switch args[0] {
	case "log", "logs":
		execFn = kubectlLogs
	}
	env := pflag.StringP("env", "e", "bindo-staging-tw", "env for kubectl")
	pflag.Parse()
	execFn(args[1], *env)
}

func kubectlLogs(arg, env string) {
	var (
		orderBy      string
		filter       string
	)
	projectID := "bindo-staging-tw"
	if strings.Contains(env, "pr") {
		projectID = "bindo-production-tw"
	}
	file, err := os.Open(os.Getenv("LOG_FILTER_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		s := scanner.Text()
		if !strings.HasPrefix(s, "#") {
			if len(s) > 0 {
				ss := strings.Split(s, "=")
				if strings.Contains(ss[0], "log.orderBy.timestamp") {
					orderBy = "timestamp " + ss[1]
					continue
				}
				if strings.Contains(s, "refresh_token") {
					refreshToken = s[strings.Index(s, ":")+1:]
					continue
				} else if strings.Contains(s, "access_token") {
					accessToken = s[strings.Index(s, ":")+1:]
					continue
				}
			}
			filter += s + "\n"
		}
	}
	filter = strings.Replace(filter, "$env", projectID, -1)
	fmt.Println(filter)
	au := &auth{}
	ctx := context.Background()
	client, err := logv2.NewClient(ctx, option.WithTokenSource(au))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	entriesReq := &loggingpb.ListLogEntriesRequest{
		//ProjectIds:    []string{"projects/bindo-staging-tw"},
		ResourceNames: []string{fmt.Sprintf("projects/%s", projectID)},
		Filter:        string(filter),
		OrderBy:       orderBy,
		PageSize:      1000,
		PageToken:     "",
	}
	entriesIt := client.ListLogEntries(ctx, entriesReq)
	index := 0
	for {
		resp, err := entriesIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Failed to ListLogEntries : %v", err)
		}
		podID := resp.Resource.Labels["pod_id"]
		if arg != "all" && !strings.HasPrefix(podID, arg) {
			//fmt.Printf("--%d--%s\n",index,podID)
			continue
		}
		index++
		fmt.Printf("%d------%s - %s-----\n", index, time.Unix(resp.Timestamp.Seconds, 0).Format("2006-01-02 15:04:05"), podID)
		jsonPayload := resp.GetJsonPayload()
		if jsonPayload != nil {
			//fmt.Println(jsonPayload)
			m := parseJson(jsonPayload)
			b, err := json.MarshalIndent(m, "    ", " ")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(b))
		} else {
			fmt.Println(resp.GetTextPayload())
		}
	}
}

func parseJson(p *structpb.Struct) map[string]interface{} {
	jsonMap := make(map[string]interface{})
	for key, value := range p.Fields {
		jsonMap[key] = formatPbValue(value)
	}
	return jsonMap
}

func formatPbValue(value *structpb.Value) interface{} {
	kind := value.GetKind()
	if kind == nil {
		return value.String()
	}
	switch v := kind.(type) {
	case *structpb.Value_NullValue:
		return v.NullValue.String()
	case *structpb.Value_NumberValue:
		return strconv.FormatFloat(v.NumberValue, 'f', -1, 64)
	case *structpb.Value_StringValue:
		return v.StringValue
	case *structpb.Value_BoolValue:
		return fmt.Sprint(v.BoolValue)
	case *structpb.Value_StructValue:
		jsonMap := make(map[string]interface{})
		for kk, vv := range v.StructValue.Fields {
			jsonMap[kk] = formatPbValue(vv)
		}
		return jsonMap
	case *structpb.Value_ListValue:
		var list []interface{}
		for _, vv := range v.ListValue.Values {
			list = append(list, formatPbValue(vv))
		}
		return list
	default:
		fmt.Printf("@@@@@@@@@@@%s", value.String())
		return value.String()
	}
}

const (
	FIELDS       = "fields:"
	KEY          = "key:"
	VALUE        = "value:"
	STRING_VALUE = "string_value:"
	STRUCT_VALUE = "struct_value:"
	BOOL_VALUE   = "bool_value:"
	NUMBER_VALUE = "number_value:"
	NULL_VALUE   = "null_value:"
	LIST_VALUE   = "list_value:"
	VALUES       = "values:"
)

func forFunc(scanner *bufio.Scanner) (bool, interface{}) {
	b := scanner.Scan()
	if !b {
		return false, nil
	}
	content := scanner.Text()
	fmt.Printf("---%s\n", content)
	if len(content) == 0 {
		return true, nil
	}
	//fmt.Println("-----: ", content)
	time.Sleep(time.Second)
	switch true {
	case strings.HasSuffix(content, FIELDS):
		return true, nil
	case strings.HasPrefix(content, KEY):
		valMap := make(map[string]interface{})
		s := strings.TrimPrefix(content, KEY)
		e := strings.Index(s, VALUE)
		k := s[1 : e-2]
		_, valMap[k] = forFunc(scanner)
		return true, valMap
	case strings.HasSuffix(content, VALUE):
		return true, nil
	case strings.HasPrefix(content, STRING_VALUE):
		s := strings.TrimPrefix(content, STRING_VALUE)
		//fmt.Println("string_value: ",s[1 : len(s)-2])
		return true, s[1 : len(s)-2]
	case strings.HasPrefix(content, STRUCT_VALUE):

	case strings.HasPrefix(content, BOOL_VALUE):
		s := strings.TrimPrefix(content, BOOL_VALUE)
		return true, strings.TrimSpace(s)
	case strings.HasPrefix(content, NUMBER_VALUE):
		s := strings.TrimPrefix(content, NUMBER_VALUE)
		//fmt.Println("number_value: ",strings.TrimSpace(s))
		return true, strings.TrimSpace(s)
	case strings.HasPrefix(content, NULL_VALUE):
	case strings.HasPrefix(content, LIST_VALUE):
	case strings.HasSuffix(content, VALUES):

	}
	return true, nil
}

func Scan(s string) {
	scan := bufio.NewScanner(strings.NewReader(s))
	scan.Split(jsonScan)
	for {
		b, out := forFunc(scan)
		if !b {
			break
		}
		switch v := out.(type) {
		case map[string]string:
			for key, value := range v {
				fmt.Printf("[%s : %s] ", key, value)
			}
			fmt.Println("")
		case map[string]interface{}:
			for key, value := range v {
				fmt.Printf("[%s : %+v] ", key, value)
			}
			fmt.Println("")
		default:
			fmt.Println("unknow type")
		}
	}
}

func jsonScan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	left := bytes.Index(data, []byte("<"))
	right := bytes.Index(data, []byte(">"))
	//fmt.Printf("left:%d right:%d\n", left, right)
	if right == -1 {
		return 0, nil, fmt.Errorf("the end")
	}
	if left < right && left != -1 {
		return left + 1, data[:left], nil
	} else {
		return right + 1, data[:right], nil
	}
}
