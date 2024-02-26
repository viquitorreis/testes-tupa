package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Iuptec/tupa"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	googleOauthConfig = &oauth2.Config{
		ClientID:     "973195558777-pa8nbjs60nu2esgahv9283agboefumb0.apps.googleusercontent.com",
		ClientSecret: "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",
		RedirectURL:  "http://localhost:6969/auth/google/callback",
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
)

func googleAuth() {

	url := googleOauthConfig.AuthCodeURL("state")
	println(url)

	tok, err := googleOauthConfig.Exchange(context.Background(), "authorization-code")
	if err != nil {
		log.Fatal(err)
	}

	client := googleOauthConfig.Client(context.Background(), tok)
	resp, err := client.Get("https://www.googleapis.com/some-api-endpoint")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Process the response
	// Example: read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(body))
}

// func HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
// 	HandleLogin(w, r, googleOauthConfig, oauthStateStringGl)
// }

func HandleLogin(tc *tupa.TupaContext) error {
	URL, err := url.Parse(googleOauthConfig.Endpoint.AuthURL)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	parameters := url.Values{}
	parameters.Add("client_id", googleOauthConfig.ClientID)
	parameters.Add("scope", strings.Join(googleOauthConfig.Scopes, " "))
	parameters.Add("redirect_uri", googleOauthConfig.RedirectURL)
	parameters.Add("response_type", "code")

	URL.RawQuery = parameters.Encode()
	url := URL.String()
	fmt.Println(googleOauthConfig.RedirectURL)

	http.Redirect((*tc.Response()), tc.Request(), url, http.StatusTemporaryRedirect)
	return nil
}

type GoogleResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Picture       string `json:"picture"`
	HostedDomain  string `json:"hd"`
}

func handleCallBackGoogle(tc *tupa.TupaContext) error {
	resp, err := tupa.AuthGoogleCallback(*tc.Response(), tc.Request())
	if err != nil {
		return err
	}
	fmt.Println(resp)
	fmt.Println("Access token:", resp.Token.AccessToken)
	fmt.Println("Access token:", resp.Token)
	fmt.Println("Access token:", resp.Token.Expiry)
	fmt.Println("Token type:", resp.Token.TokenType)

	tc.SendString("Hello world!")
	return nil
}

type GoogleAuthResponse struct {
	UserInfo tupa.GoogleDefaultResponse
	Token    *oauth2.Token
}

func CallBackFromGoogle(w http.ResponseWriter, r *http.Request) (*GoogleAuthResponse, error) {

	code := r.FormValue("code")

	if code == "" {
		w.Write([]byte("Usuário não aceitou a autenticação...\n"))
		reason := r.FormValue("error_reason")
		if reason == "user_denied" {
			w.Write([]byte("Usuário negou a permissão..."))
		}

		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return nil, nil
	}

	token, err := tupa.GoogleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		fmt.Printf("Exchange do código falhou '%s'\n", err)
		return nil, err
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + url.QueryEscape(token.AccessToken))
	if err != nil {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return nil, nil
	}
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return nil, nil
	}
	fmt.Println(string(response))

	var userInfo tupa.GoogleDefaultResponse
	err = json.Unmarshal(response, &userInfo)
	if err != nil {
		return nil, err
	}

	return &GoogleAuthResponse{
		UserInfo: userInfo,
		Token:    token,
	}, nil
}

func ConfigGoogle(tc *tupa.TupaContext) {
	godotenv.Load()
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	tupa.UseGoogleOauth(clientID, clientSecret, "http://localhost:6969/auth/google/callback", []string{"https://www.googleapis.com/auth/userinfo.email"})
	tupa.AuthGoogleHandler(tc)
}

// func handlingGoauth(tc *tupa.TupaContext) error {
// 	fmt.Println("Handling Google Auth")
// 	fmt.Println(tupa.GoogleOauthConfig)
// 	resp := tupa.AuthGoogleCallback(*tc.Response(), tc.Request())
// 	fmt.Println(resp)
// 	var userInfo GoogleResponse
// 	err := json.Unmarshal([]byte(resp), &userInfo)
// 	if err != nil {
// 		fmt.Println("Error:", err)
// 		return nil
// 	}

// 	fmt.Println(resp)
// 	fmt.Println("ID:", userInfo.ID)
// 	fmt.Println("Email:", userInfo.Email)
// 	fmt.Println("Verified Email:", userInfo.VerifiedEmail)
// 	fmt.Println("Picture:", userInfo.Picture)
// 	fmt.Println("Hosted Domain:", userInfo.HostedDomain)
// 	return nil
// }
