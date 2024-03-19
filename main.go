package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/big"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"syscall"
	"time"

	// pages "https://github.com/viquitorreis/testes-tupa/pages"

	"github.com/Iuptec/tupa"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func FmtBlue(s string) string {
	return "\033[1;34m" + s + "\033[0m"
}

func FmtRed(s string) string {
	return string("\033[1;31m") + s + string("\033[0m")
}

func FmtYellow(s string) string {
	return string("\033[1;33m ") + s + string("\033[0m")
}

type (
	Context interface {
		Request() *http.Request
		Response() http.ResponseWriter
		SendString(s string) error
		Param(param string) string
		QueryParam(param string) string
		QueryParams() map[string][]string
	}

	TupaContext struct {
		request  *http.Request
		response http.ResponseWriter
		context.Context
	}
)

type APIFunc func(*TupaContext) error

type APIServer struct {
	listenAddr        string
	server            *http.Server
	globalMiddlewares MiddlewareChain
	router            *mux.Router
}

type HTTPMethod string

type APIError struct {
	Error string
}

const (
	MethodGet     HTTPMethod = http.MethodGet
	MethodPost    HTTPMethod = http.MethodPost
	MethodPut     HTTPMethod = http.MethodPut
	MethodDelete  HTTPMethod = http.MethodDelete
	MethodPatch   HTTPMethod = http.MethodPatch
	MethodOptions HTTPMethod = http.MethodOptions
)

var AllowedMethods = map[HTTPMethod]bool{
	MethodGet:    true,
	MethodPost:   true,
	MethodPut:    true,
	MethodDelete: true,
	MethodPatch:  true,
}

type RouteInfo struct {
	Path        string
	Method      HTTPMethod
	Handler     APIFunc
	Middlewares []MiddlewareFunc
}

func (a *APIServer) New() {
	if a.router.GetRoute("/") == nil {
		a.RegisterRoutes([]RouteInfo{
			{
				Path:    "/",
				Method:  MethodGet,
				Handler: WelcomeHandler,
			},
		})
	}

	// c := cors.New(cors.Options{
	// 	// AllowedOrigins:   []string{"http://localhost:4200", "http://localhost:6969"},
	// 	// AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions, http.MethodPut, http.MethodPatch, http.MethodHead, http.MethodConnect, http.MethodTrace},
	// 	// AllowedHeaders:   []string{"*"},
	// 	AllowCredentials: true,
	// })

	c := cors.New(cors.Options{
		AllowedHeaders: []string{
			"Authorization", "authorization", "Accept", "Content-Type", "X-Requested-With", "X-Frame-Options",
			"X-XSS-Protection", "X-Content-Type-Options", "X-Permitted-Cross-Domain-Policies", "Referrer-Policy", "Expect-CT",
			"Feature-Policy", "Content-Security-Policy", "Content-Security-Policy-Report-Only", "Strict-Transport-Security",
			"Public-Key-Pins", "Public-Key-Pins-Report-Only", "Access-Control-Allow-Origin", "Access-Control-Allow-Methods",
			"Access-Control-Allow-Headers", "Access-Control-Allow-Credentials", "X-Forwarded-For", "X-Real-IP",
			"X-Csrf-Token", "X-HTTP-Method-Override",
		},
		AllowCredentials: true,
	})

	// routerHandler := cors.Default().Handler(a.router)

	// testingHandler := &cors.Cors{

	// }
	routerHandler := c.Handler(a.router)

	// a.router.Use(accessControlMiddleware)

	a.server = &http.Server{
		Addr:    a.listenAddr,
		Handler: routerHandler,
	}

	fmt.Println(FmtBlue("Servidor iniciado na porta: " + a.listenAddr))

	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(FmtRed("Erro ao iniciar servidor: "), err)
		}
		log.Println(FmtYellow("Servidor parou de receber novas conexões"))
	}()

	signchan := make(chan os.Signal, 1)
	signal.Notify(signchan, syscall.SIGINT, syscall.SIGTERM)
	<-signchan // vai esperar um comando que encerra o servidor

	ctx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Fatal(FmtRed("Erro ao desligar servidor: "), err)
	}

	fmt.Println(FmtYellow("Servidor encerrado na porta: " + a.listenAddr))
}

func NewAPIServer(listenAddr string) *APIServer {
	router := mux.NewRouter()
	// router.Use(accessControlMiddleware)

	return &APIServer{
		listenAddr:        listenAddr,
		router:            router,
		globalMiddlewares: MiddlewareChain{},
	}
}

// func accessControlMiddleware(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200/")
// 		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200")
// 		// w.Header().Add("Access-Control-Allow-Methods", "http://localhost:4200")
// 		// w.Header().Add("Access-Control-Allow-Methods", "localhost:4200/")
// 		// w.Header().Add("Access-Control-Allow-Methods", "localhost:4200")
// 		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With")
// 		w.Header().Set("Access-Control-Allow-Credentials", "true")

// 		log.Println("Middleware accessControlMiddleware")

// 		if r.Method == "OPTIONS" {
// 			log.Println("OPTIONS call")
// 			w.WriteHeader(http.StatusOK)
// 			return
// 		}

// 		log.Println("Access-Control-Allow-Origin:", w.Header().Get("Access-Control-Allow-Origin"))
// 		log.Println("Access-Control-Allow-Methods:", w.Header().Get("Access-Control-Allow-Methods"))
// 		log.Println("Access-Control-Allow-Headers:", w.Header().Get("Access-Control-Allow-Headers"))
// 		log.Println("Access-Control-Allow-Credentials:", w.Header().Get("Access-Control-Allow-Credentials"))

// 		next.ServeHTTP(w, r)
// 	})
// }

// func MiddlewareContrBTupa(next tupa.APIFunc) tupa.APIFunc {
// 	return func(tc *tupa.TupaContext) error {
// 		fmt.Println("Middleware MiddlewareContrB")
// 		return next(tc)
// 	}
// }

func WelcomeHandler(tc *TupaContext) error {
	WriteJSONHelper(tc.response, http.StatusOK, "Seja bem vindo ao Tupã framework!")
	return nil
}

func (a *APIServer) RegisterRoutes(routeInfos []RouteInfo) {
	for _, routeInfo := range routeInfos {
		if !AllowedMethods[routeInfo.Method] {
			log.Fatalf(fmt.Sprintf(FmtRed("Método HTTP não permitido: "), "%s\nVeja como criar um novo método na documentação", routeInfo.Method))
		}

		var allMiddlewares MiddlewareChain
		middlewaresGlobais := a.GetGlobalMiddlewares()

		allMiddlewares = append(allMiddlewares, middlewaresGlobais...)

		handler := a.MakeHTTPHandlerFuncHelper(routeInfo, allMiddlewares, a.globalMiddlewares)

		a.router.HandleFunc(routeInfo.Path, handler).Methods(string(routeInfo.Method))
	}
}

func WriteJSONHelper(w http.ResponseWriter, status int, v any) error {
	if w == nil {
		return errors.New("Response writer passado está nulo")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(v)
}

func (a *APIServer) MakeHTTPHandlerFuncHelper(routeInfo RouteInfo, middlewares MiddlewareChain, globalMiddlewares MiddlewareChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := &TupaContext{
			request:  r,
			response: w,
		}

		// Combina middlewares globais com os especificos de rota
		allMiddlewares := MiddlewareChain{}
		allMiddlewares = append(allMiddlewares, a.GetGlobalMiddlewares()...)
		allMiddlewares = append(allMiddlewares, routeInfo.Middlewares...)

		doneCh := a.executeMiddlewaresAsync(ctx, allMiddlewares)
		errorsSlice := <-doneCh // espera até que algum valor seja recebido. Continua no primeiro erro recebido ( se houver ) ou se não houver nenhum erro

		if len(errorsSlice) > 0 {
			WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: errorsSlice[0].Error()})
			return
		}

		log.Println(r.Method, r.URL.Path)

		if r.Method == string(routeInfo.Method) {
			if err := routeInfo.Handler(ctx); err != nil {
				if err := WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()}); err != nil {
					fmt.Println("Erro ao escrever resposta JSON:", err)
				}
			}

		} else {
			WriteJSONHelper(w, http.StatusMethodNotAllowed, APIError{Error: "Método HTTP não permitido"})
		}
	}
}

func (tc *TupaContext) Request() *http.Request {
	return tc.request
}

func (tc *TupaContext) Response() *http.ResponseWriter {
	return &tc.response
}

func (tc *TupaContext) SendString(s string) error {
	_, err := tc.response.Write([]byte(s))
	return err
}

func (tc *TupaContext) Param(param string) string {
	return mux.Vars(tc.request)[param]
}

func (tc *TupaContext) QueryParam(param string) string {
	return tc.request.URL.Query().Get(param)
}

func (tc *TupaContext) QueryParams() map[string][]string {
	return tc.request.URL.Query()
}

func (tc *TupaContext) NewTupaContext(w http.ResponseWriter, r *http.Request) *TupaContext {
	return &TupaContext{
		request:  r,
		response: w,
	}
}

// TESTES HANDLERS

func HandleAPIEndpoint(tc *TupaContext) error {
	fmt.Println("Endpoint da API")
	WriteJSONHelper(tc.response, http.StatusOK, "Endpoint da API")
	return nil
}

func HandleAPIEndpoint3(tc *TupaContext) error {
	fmt.Println("Endpoint da HandleAPIEndpoint3")
	WriteJSONHelper(tc.response, http.StatusOK, "Endpoint da HandleAPIEndpoint3")
	return nil
}

func HandleAPIEndpoint2(w http.ResponseWriter, r *http.Request) error {
	fmt.Println("Hello world! Endpoint da API 2")
	WriteJSONHelper(w, http.StatusOK, "Endpoint da API")
	return nil
}

func HandlePOSTEndpoint(tc *TupaContext) error {
	var testeString string

	if err := json.NewDecoder(tc.request.Body).Decode(&testeString); err != nil {
		return err
	}

	fmt.Println(testeString)
	time.Sleep(time.Second * 5)
	fmt.Println(FmtBlue("Depois de 5s do shutdown"))

	WriteJSONHelper(tc.response, http.StatusOK, testeString)
	return nil
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func handleSendString(tc *TupaContext) error {
	return tc.SendString("Hello world")
}

func HandleEndpointQueryParam(tc *TupaContext) error {
	param := tc.QueryParam("name")
	tc.SendString("Hello " + param)
	return nil
}

func HandleGetParam(tc *tupa.TupaContext) error {
	param := tc.QueryParams()
	tc.SendString("Hello " + param["name"][0])
	return nil
}

func HandleEndpointQueryParams(tc *TupaContext) error {
	param := tc.QueryParams()
	fmt.Println(param)
	fmt.Println(param["idade"])
	fmt.Println(param["name"])
	tc.SendString("Hello " + param["name"][0])
	return nil
}

func PrintRoutes(router *mux.Router) {
	router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathTemplate, err := route.GetPathTemplate()
		if err == nil {
			methods, _ := route.GetMethods()
			fmt.Printf("\nRoute: %s\n Metodos: %s", pathTemplate, methods)
		}

		return nil
	})
}

// MIDDLEWARES EXEMPLOS

func LoggingMiddleware(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		log.Println("Antes de chamar o handler")
		defer log.Println("Depois de chamar o handler")

		tupaContextWithValue := context.WithValue(tc.request.Context(), "ctxText", "2602")

		tc.request = tc.request.WithContext(tupaContextWithValue)
		// chamando o handler original
		return next(tc)
	}
}

func HelloWorldMiddleware(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		log.Println("Hello world Antes de chamar o handler")
		defer log.Println("Hello world fim")

		// chamando o handler original
		return next(tc)
	}
}

func LoggingMiddlewareAfter(next APIFunc) APIFunc {
	fmt.Println("Middleware LoggingMiddlewareAfter")
	afterHandler := func(tc *TupaContext) error {
		defer log.Println("Depois de chamar o handler 2")
		return next(tc)
	}

	return afterHandler
}

func LoggingMiddlewareWithErrorrr(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {

		// errMsg := errors.New("erro no middleware LoggingMiddlewareWithError")
		// err := WriteMiddErrorJSONHelper(tc, errMsg)
		// tc.response.WriteHeader(http.StatusInternalServerError)

		// escrevendo o erro no body da req
		// tc.response.Write([]byte(`{"Error":"` + "houve um erro" + `"}`))

		ctx := context.WithValue(tc.request.Context(), "smpErrorMidd", "sampleErrorMiddleware")
		tc.request = tc.request.WithContext(ctx)

		return errors.New("erro no middleware LoggingMiddlewareWithError")
	}
}

func MiddlewareAPIEndpoint(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("Middleware API Endpoint GLOBAL")
		return next(tc)
	}
}

func MiddlewareContrA(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("Middleware MiddlewareContrA")
		return next(tc)
	}
}

func MiddlewareContrB(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("Middleware MiddlewareContrB")
		return next(tc)
	}
}

func MiddlewareContrC(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("Middleware MiddlewareContrC")
		return next(tc)
	}
}

func MiddlewarePost(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("MiddlewarePost")
		return next(tc)
	}
}

func MiddlewareSampleErr(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("MiddlewareSampleErr")
		return errors.New("Erro no middleware MiddlewareSampleErr")
	}
}
func MiddlewareSampleErr2(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("MiddlewareSampleErr2")
		return errors.New("Erro no middleware MiddlewareSampleErr2")
	}
}

func MiddlewareGLOBAL(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("MiddlewareGLOBAL")
		return next(tc)
	}
}

// user
func main() {
	server := NewAPIServer(":6968")

	ExampleRouteManagerTupa()
	// AddRoutes(nil, ContrTestAuthCors)
	server.RegisterRoutes(GetRoutes())

	server.New()
}

func GetFileHandler(tc *TupaContext) error {
	const page = "upload"
	displayTemplate(tc, page, nil)

	return nil
}

func UploadFileHandler(tc *TupaContext) error {
	filHeader, err := UploadFile(tc, "image", "static/images/", "arquivo_qualquer")
	if err != nil {
		return err
	}

	fmt.Println(filHeader.Filename)
	return nil
}

func displayTemplate(tc *TupaContext, page string, data interface{}) {
	var templates = template.Must(template.ParseFiles("pages/upload.html"))
	templates.ExecuteTemplate(*tc.Response(), page+".html", data)
}

func UploadFile(tc *TupaContext, filePrefix, destFolder, formFileKey string) (multipart.FileHeader, error) {
	tc.Request().ParseMultipartForm(10 << 20)

	file, fileHeader, err := tc.Request().FormFile(formFileKey)
	if err != nil {
		fmt.Println("Erro ao retornar o arquivo")
		fmt.Println(err)
		return multipart.FileHeader{}, err
	}

	randStr, err := GenerateRandomString(6)
	if err != nil {
		return multipart.FileHeader{}, err
	}
	fileHeader.Filename = filePrefix + "_" + randStr + fileHeader.Filename

	defer file.Close()
	// fmt.Printf("Uploaded File: %+v\n", fileHeader.Filename)
	// fmt.Printf("File Size: %+v\n", fileHeader.Size)
	// fmt.Printf("MIME Header: %+v\n", fileHeader.Header)

	destPath := filepath.Join(destFolder, fileHeader.Filename)
	destFile, err := os.Create(destPath)
	if err != nil {
		return multipart.FileHeader{}, err
	}

	defer destFile.Close()

	// copia o arquivo do upload para o arquivo criado no SO
	if _, err := io.Copy(destFile, file); err != nil {
		return multipart.FileHeader{}, WriteJSONHelper(*tc.Response(), http.StatusInternalServerError, err.Error())
	}

	fmt.Fprint(*tc.Response(), "Arquivo salvo com sucesso\n")
	return *fileHeader, nil
}

func GenerateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetLen := big.NewInt(int64(len(charset)))

	randomString := make([]byte, length)
	for i := range randomString {
		randomIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		randomString[i] = charset[randomIndex.Int64()]
	}
	return string(randomString), nil
}

// ////////// testes pkt tupa
func MiddlewareGLOBALTupa(next tupa.APIFunc) tupa.APIFunc {
	return func(tc *tupa.TupaContext) error {
		fmt.Println("MiddlewareGLOBAL")
		return next(tc)
	}
}

func HandleAPIEndpointTupa(tc *tupa.TupaContext) error {
	fmt.Println("Endpoint da API")
	tupa.WriteJSONHelper(*tc.Response(), http.StatusOK, "Endpoint da API")
	return nil
}

func MiddlewareContrATupa(next tupa.APIFunc) tupa.APIFunc {
	return func(tc *tupa.TupaContext) error {
		fmt.Println("Middleware MiddlewareContrA")
		return next(tc)
	}
}

func MiddlewareContrBTupa(next tupa.APIFunc) tupa.APIFunc {
	return func(tc *tupa.TupaContext) error {
		fmt.Println("Middleware MiddlewareContrB")
		return next(tc)
	}
}

func MiddlewareContrCTupa(next tupa.APIFunc) tupa.APIFunc {
	return func(tc *tupa.TupaContext) error {
		fmt.Println("Middleware MiddlewareContrC")
		return next(tc)
	}
}

func ExampleRouteManagerTupa() {
	// tupa.AddRoutes(tupa.MiddlewareChain{MiddlewareContrATupa, MiddlewareContrBTupa}, ContrARoutesTupa)

	// tupa.AddRoutes(tupa.MiddlewareChain{MiddlewareContrBTupa}, ContrBRoutesTupa)

	// tupa.AddRoutes(tupa.MiddlewareChain{MiddlewareContrCTupa}, ContrCRoutesTupa)
	AddRoutes(nil, ContrUploadImage)
}

func handleSendStringTupa(tc *tupa.TupaContext) error {
	fmt.Println("handleSendStringTupa")
	return tc.SendString("Hello world oauth")
}

const IndexPage = `
<html>
	<head>
		<title>OAuth-2 Test</title>
	</head>
	<body>
		<h2>OAuth-2 Test</h2>
		<p>
			Login with the following,
		</p>
		<ul>
			<li><a href="/login-gl">Google</a></li>
		</ul>
	</body>
</html>
`

func handleMain(tc *tupa.TupaContext) error {
	(*tc.Response()).Header().Set("Content-Type", "text/html; charset=utf-8")
	(*tc.Response()).WriteHeader(http.StatusOK)
	(*tc.Response()).Write([]byte(IndexPage))

	return tc.SendString("Hello world oauth")
}

func ContrUploadImage() []RouteInfo {
	return []RouteInfo{
		{
			Path:    "/form",
			Method:  "GET",
			Handler: GetFileHandler,
		},
		{
			Path:    "/upload",
			Method:  "POST",
			Handler: UploadFileHandler,
		},
	}
}

func AuthGoogleLogin(tc *TupaContext) error {

	clientID := "328882923422-gg4m2s4druhop7fif2tro6dv7k97onk5.apps.googleusercontent.com"
	clientSecret := "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"

	apiGoogleCallbackUrl := fmt.Sprintf("%s/%s/auth/google/callback", "http://localhost:6969", "api/v1")
	UseGoogleOauth(clientID, clientSecret, apiGoogleCallbackUrl, "http://localhost:6969", []string{"https://www.googleapis.com/auth/userinfo.email"})

	if err := AuthGoogleHandler(tc); err != nil {
		return err
	}

	return nil
}

func AuthGoogleCallbackFunc(tc *TupaContext) error {

	if tc.Request().Method == "OPTIONS" {
		(*tc.Response()).WriteHeader(http.StatusOK)
		return nil
	}

	var response map[string]string
	frontUrl := os.Getenv("http://localhost:4200")

	http.Redirect(*tc.Response(), tc.Request(), fmt.Sprintf("%s/dashboard?token=%s", frontUrl, response), http.StatusFound)
	return nil
}
