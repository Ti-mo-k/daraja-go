package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type AuthResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

func createAuthKey() string{

	consumerkey := os.Getenv("CONSUMER_KEY")
	consumersecret := os.Getenv("CONSUMER_SECRET")

	credentials := consumerkey + ":" + consumersecret

	return base64.RawStdEncoding.EncodeToString([]byte(credentials))

}

var authKey = createAuthKey()

func GetAccessToken(ctx context.Context) (*AuthResponse,error){

	authURL := os.Getenv("DARAJA_OAUTH_URL")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil )
	if err != nil{
		return nil, fmt.Errorf("An error has occured %w", err)
	}

	req.Header.Set("Authorization", "Basic "+authKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil{
		return nil, fmt.Errorf("an error while responding occured: %w", err)

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK{
		return nil, fmt.Errorf("daraja returned status: %s", resp.Status)
	}

	var authResponse  AuthResponse

	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
    	return nil, fmt.Errorf("failed to decode Daraja response: %w", err)
	}

	return &authResponse, nil

}




