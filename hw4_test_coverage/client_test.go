package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type XMLUser struct {
	Id        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

func (user XMLUser) ToUser() User {
	return User{
		Id:     user.Id,
		Name:   user.FirstName + " " + user.LastName,
		Age:    user.Age,
		About:  user.About,
		Gender: user.Gender,
	}
}

type Document struct {
	XMLName xml.Name  `xml:"root"`
	Users   []XMLUser `xml:"row"`
}

type SearchServerQuery struct {
	Limit      int
	Offset     int
	Query      string
	OrderField string
	OrderBy    int
}

func getUsers(filename string) ([]User, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return []User{},
			fmt.Errorf("Failed to read file %s: %v", filename, err)
	}

	doc := Document{}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return []User{},
			fmt.Errorf("Unmarshal failed: %v", err)
	}

	users := []User{}
	for _, XMLUser := range doc.Users {
		users = append(users, XMLUser.ToUser())
	}

	return users, nil
}

const (
	testToken       = "b3dc6835-57dc-4307-9219-5eaceec47054"
	testTokenBroken = "8de107a4-e7c3-43b2-ae39-8b7f37859e28"
)

func CheckAccess(r *http.Request) bool {
	cookie := r.Header.Get("AccessToken")
	return cookie == testToken
}

func SearchServerParseQ(q url.Values) (SearchServerQuery, error) {
	parsed := SearchServerQuery{}

	// limit
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		return SearchServerQuery{},
			fmt.Errorf("q limit failed: query=%v: %v", q, err)
	}
	parsed.Limit = limit

	// offset
	offset, err := strconv.Atoi(q.Get("offset"))
	if err != nil {
		return SearchServerQuery{},
			fmt.Errorf("q offset failed: query=%v: %v", q, err)
	}
	parsed.Offset = offset

	// query
	parsed.Query = q.Get("query")

	// order_filed
	orderField := q.Get("order_field")
	availableOrderFields := map[string]struct{}{
		"id":   {},
		"age":  {},
		"name": {},
	}
	if _, exist := availableOrderFields[strings.ToLower(orderField)]; !exist {
		return SearchServerQuery{},
			fmt.Errorf("q order_field failed: query=%v", q)
	}
	parsed.OrderField = orderField

	// order_by
	orderBy, err := strconv.Atoi(q.Get("order_by"))
	if err != nil {
		return SearchServerQuery{},
			fmt.Errorf("q order_by failed: query=%v: %v", q, err)
	}
	if orderBy < -1 || orderBy > 1 {
		return SearchServerQuery{},
			fmt.Errorf("order_by out of range: query=%v", q)
	}
	parsed.OrderBy = orderBy

	return parsed, nil
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	filename := "dataset.xml"

	users, err := getUsers(filename)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		fmt.Fprintf(w, "%d: InternalServerError: %v",
			http.StatusInternalServerError,
			err)
		return
	}

	if authorized := CheckAccess(r); !authorized {
		w.WriteHeader(http.StatusUnauthorized)

		fmt.Fprintf(w, "%d: StatusUnauthorized: %v",
			http.StatusUnauthorized,
			err)
		return
	}

	params, err := SearchServerParseQ(r.URL.Query())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		fmt.Fprintf(w, "%d: StatusBadRequest: %v",
			http.StatusBadRequest,
			err)
		return
	}

	// query - filter
	filtered := []User{}
	if params.Query != "" {
		for _, user := range users {
			if strings.Contains(user.About, params.Query) ||
				strings.Contains(user.Name, params.Query) {
				filtered = append(filtered, user)
			}
		}
		users = filtered
	}
	// order_by, order_field
	sortFuncs := map[string]map[int]func(i, j int) bool{}
	sortFuncs["Id"] = map[int]func(i, j int) bool{}
	sortFuncs["Id"][OrderByAsc] = func(i, j int) bool { return users[i].Id > users[j].Id }
	sortFuncs["Id"][OrderByDesc] = func(i, j int) bool { return !sortFuncs["Id"][OrderByAsc](i, j) }
	sortFuncs["Age"] = map[int]func(i, j int) bool{}
	sortFuncs["Age"][OrderByAsc] = func(i, j int) bool { return users[i].Age > users[j].Age }
	sortFuncs["Age"][OrderByDesc] = func(i, j int) bool { return !sortFuncs["Age"][OrderByAsc](i, j) }
	sortFuncs["Name"] = map[int]func(i, j int) bool{}
	sortFuncs["Name"][OrderByAsc] = func(i, j int) bool { return users[i].Name > users[j].Name }
	sortFuncs["Name"][OrderByDesc] = func(i, j int) bool { return !sortFuncs["Name"][OrderByAsc](i, j) }
	if params.OrderBy == OrderByAsc || params.OrderBy == OrderByDesc {
		sort.Slice(users, sortFuncs[params.OrderField][params.OrderBy])
	}
	// offset
	min := func(l, r int) int {
		if l < r {
			return l
		}
		return r
	}
	users = users[min(params.Offset, len(users)):]
	// limit
	users = users[:min(params.Limit, len(users))]

	js, err := json.Marshal(users)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		fmt.Fprintf(w, "%d: StatusInternalServerError: %v",
			http.StatusInternalServerError,
			err)
		return
	}

	w.Write(js)
}

//////////////////////////////////////////////////////////////////////////////

func TestUnmarshal(t *testing.T) {
	filename := "dataset.xml"

	users, err := getUsers(filename)

	if err != nil {
		t.Errorf("getUsers failed: %v", err)
		return
	}

	if len(users) != 35 {
		t.Errorf("Incorrect slice len: 35 vs %d",
			len(users))
		return
	}

	firstUser := User{
		Id:   0,
		Name: "Boyd Wolf",
		Age:  22,
		About: `Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.
`,
		Gender: "male",
	}

	if users[0] != firstUser {
		t.Errorf("First users is diff: %v vs %v", firstUser, users[0])
	}
}

///////////////////////////////////////////////////////////////////////////////

const (
	ErrorLimitLessZero  = "limit must be > 0"
	ErrorOffsetLessZero = "offset must be > 0"
	ErrorIncorQueryGUID = "q order_field failed: query=GUID"
	ErrorBadAccessToken = "Bad AccessToken"
	ErrorInternalServer = "SearchServer fatal error"
	ErrorUnknownError   = "unknown error"
)

func TestPosDefault(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if err != nil {
		t.Errorf("Unexpected error on FindUsers: %v", err)
		return
	}

	for _, user := range resp.Users {
		if !(strings.Contains(user.About, request.Query) ||
			strings.Contains(user.Name, request.Query)) {
			t.Errorf("query isnot in resp; user.About = %v; user.Name = $%v$; query = $%v$",
				user.About, user.Name, request.Query)
			return
		}
	}
}

func TestNegLimitLessZero(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      -10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response shoult be nil: %v", resp)
		return
	}
	if err.Error() != fmt.Errorf(ErrorLimitLessZero).Error() {
		t.Errorf("incorrect err mes: %v vs %v",
			ErrorLimitLessZero, err)
	}
}

func TestNegMore25(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      26,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if err != nil {
		t.Errorf("Unexpected error on FindUsers: %v", err)
		return
	}

	for _, user := range resp.Users {
		if !(strings.Contains(user.About, request.Query) ||
			strings.Contains(user.Name, request.Query)) {
			t.Errorf("query isnot in resp; user.About = %v; user.Name = $%v$; query = $%v$",
				user.About, user.Name, request.Query)
			return
		}
	}
}

func TestNegOffsetLessZero(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     -10,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response shoult be nil: %v", resp)
		return
	}
	if err.Error() != fmt.Errorf(ErrorOffsetLessZero).Error() {
		t.Errorf("incorrect err mes: %v vs %v",
			ErrorOffsetLessZero, err)
	}
}

func TestNegTimeout(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			SearchServer(w, r)
		}))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response shoult be nil: %v", resp)
		return
	}
	if err == nil {
		t.Errorf("should be an error")
		return
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("incorrect err mes: %v vs %v",
			ErrorOffsetLessZero, err)
		return
	}
}

func TestNegEmptyURL(t *testing.T) {
	searchClient := &SearchClient{testToken, ""}

	resp, err := searchClient.FindUsers(SearchRequest{})

	if resp != nil {
		t.Errorf("response should be nil: %v", resp)
		return
	}
	if err == nil || !strings.Contains(err.Error(), ErrorUnknownError) {
		t.Errorf("incorrect err mes: %v vs %v",
			ErrorUnknownError+"...", err)
		return
	}
}

func TestNegIncorAccessToken(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "GUID",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testTokenBroken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response shoult be nil: %v", resp)
		return
	}
	if err.Error() != fmt.Errorf(ErrorBadAccessToken).Error() {
		t.Errorf("incorrect err mes: %v vs %v",
			ErrorBadAccessToken, err)
		return
	}
}

func TestNegInternalServerError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response should be nil")
		return
	}
	if err == nil || err.Error() != fmt.Errorf(ErrorInternalServer).Error() {
		t.Errorf("incorrect err mes: %v vs %v",
			ErrorInternalServer, err)
		return
	}
}

func TestNegInternalServerErrorBadRequestIncorJs(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Incorrect json"))
		}))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response should be nil")
		return
	}
	if err == nil || !strings.Contains(err.Error(), "cant unpack error json") {
		t.Errorf("incorrect err mes: %v vs %v",
			"cant unpack error json ...", err)
		return
	}
}

func TestNegInternalServerErrorBadOrderField(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"ErrorBadOrderField"}`))
		}))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response should be nil")
		return
	}
	if err == nil || !strings.Contains(err.Error(), "OrderFeld") {
		t.Errorf("incorrect err mes: %v vs %v",
			"OrderFeld ...", err)
		return
	}
}

func TestNegInternalServerErrorUnknownError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Unknown Error"}`))
		}))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response should be nil")
		return
	}
	if err == nil ||
		!strings.Contains(err.Error(), "unknown bad request error: ") {
		t.Errorf("incorrect err mes: %v vs %v",
			"unknown bad request error:  ...", err)
		return
	}
}

func TestNegIncorJs(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Incorrect json"))
		}))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      10,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if resp != nil {
		t.Errorf("response should be nil")
		return
	}
	if err == nil || !strings.Contains(err.Error(), "cant unpack result json:") {
		t.Errorf("incorrect err mes: %v vs %v",
			"cant unpack result json: ...", err)
		return
	}
}

func TestPosNextPage(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer testServer.Close()

	request := SearchRequest{
		Limit:      0,
		Offset:     0,
		Query:      "Dillard",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	searchClient := SearchClient{
		AccessToken: testToken,
		URL:         testServer.URL,
	}
	resp, err := searchClient.FindUsers(request)
	if err != nil {
		t.Errorf("Unexpected error on FindUsers: %v", err)
		return
	}

	for _, user := range resp.Users {
		if !(strings.Contains(user.About, request.Query) ||
			strings.Contains(user.Name, request.Query)) {
			t.Errorf("query isnot in resp; user.About = %v; user.Name = $%v$; query = $%v$",
				user.About, user.Name, request.Query)
			return
		}
	}
}
