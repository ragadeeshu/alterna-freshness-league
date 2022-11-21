package datahandling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/anaskhan96/soup"
)

type bulletApiTokenResponse struct {
	BulletToken string `json:"bulletToken"`
}
type nintendoApiTokenResponse struct {
	AccessToken string `json:"access_token"`
	IdToken     string `json:"id_token"`
}
type splatoonApiTokenResponse struct {
	Result struct {
		User struct {
			ImageUri string `json:"imageUri"`
			Name     string `json:"name"`
		} `json:"user"`
		WebApiServerCredential struct {
			AccessToken string `json:"accessToken"`
		} `json:"webApiServerCredential"`
	} `json:"result"`
}
type webServiceApiTokenResponse struct {
	Result struct {
		AccessToken string `json:"accessToken"`
	} `json:"result"`
}
type nintendoApiUserInfoResponse struct {
	Nickname string `json:"nickname"`
	Language string `json:"language"`
	Country  string `json:"country"`
	Birthday string `json:"birthday"`
}
type iminkApiResponse struct {
	Ftoken    string `json:"f"`
	RequestId string `json:"request_id"`
	Timestamp int    `json:"timestamp"`
}

func splatnetLogin(contestant *Contestant, nsoAppVersion string, webveiwVersion string, client *http.Client) (*splatnetAccount, error) {
	nintendoTokenResponse, err := getNintendoAccessToken(contestant, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get nintendoTokenResponse: %w", err)
	}

	nintendoUserInfo, err := getNintendoUserInfo(nintendoTokenResponse, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get nintendoUserInfo: %w", err)
	}

	iminkResponse1, err := callImink(nintendoTokenResponse.IdToken, 1, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get iminkResponse1: %w", err)
	}

	splatoonTokenResponse, err := getSplatoonAccessToken(iminkResponse1, nintendoUserInfo, nintendoTokenResponse, nsoAppVersion, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get splatoonTokenResponse: %w", err)
	}

	iminkResponse2, err := callImink(splatoonTokenResponse.Result.WebApiServerCredential.AccessToken, 2, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get iminkResponse2: %w", err)
	}

	webServiceTokenResponse, err := getWebServiceToken(iminkResponse2, splatoonTokenResponse, nsoAppVersion, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get webServiceTokenResponse: %w", err)
	}

	bulletTokenResponse, err := getBulletToken(nintendoUserInfo, webveiwVersion, webServiceTokenResponse, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get bulletTokenResponse: %w", err)
	}

	graphQlHeader := map[string]string{
		"Authorization":    fmt.Sprintf("Bearer %s", bulletTokenResponse.BulletToken),
		"Accept-Language":  nintendoUserInfo.Language,
		"User-Agent":       "Mozilla/5.0 (Linux; Android 11; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Mobile Safari/537.36",
		"X-Web-View-Ver":   webveiwVersion,
		"Content-Type":     "application/json",
		"Accept":           "*/*",
		"Origin":           "https://api.lp1.av5ja.srv.nintendo.net",
		"X-Requested-With": "com.nintendo.znca",
		"Referer":          fmt.Sprintf("https://api.lp1.av5ja.srv.nintendo.net/?lang=%s&na_country=%s&na_lang=%s", nintendoUserInfo.Language, nintendoUserInfo.Country, nintendoUserInfo.Language),
	}

	return &splatnetAccount{
		nsoName:       nintendoUserInfo.Nickname,
		nsoImage:      splatoonTokenResponse.Result.User.ImageUri,
		accessToken:   webServiceTokenResponse.Result.AccessToken,
		graphQlHeader: graphQlHeader,
	}, nil
}

func getBulletToken(nintendoUserInfo *nintendoApiUserInfoResponse, webveiwVersion string, webServiceTokenResponse *webServiceApiTokenResponse, client *http.Client) (*bulletApiTokenResponse, error) {
	requestURL := "https://api.lp1.av5ja.srv.nintendo.net/api/bullet_tokens"
	req, err := http.NewRequest(http.MethodPost, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", nintendoUserInfo.Language)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 11; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Mobile Safari/537.36")
	req.Header.Set("X-Web-View-Ver", webveiwVersion)
	req.Header.Set("X-NACOUNTRY", nintendoUserInfo.Country)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://api.lp1.av5ja.srv.nintendo.net")
	req.Header.Set("X-Requested-Withn", "com.nintendo.znca")
	req.AddCookie(&http.Cookie{
		Name:  "_gtoken",
		Value: webServiceTokenResponse.Result.AccessToken,
	})
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()
	var bulletTokenResponse bulletApiTokenResponse
	err = json.NewDecoder(response.Body).Decode(&bulletTokenResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json from response: %w", err)
	}
	return &bulletTokenResponse, nil
}

func getWebServiceToken(iminkResponse2 *iminkApiResponse, splatoonTokenResponse *splatoonApiTokenResponse, nsoAppVersion string, client *http.Client) (*webServiceApiTokenResponse, error) {
	body := map[string]interface{}{
		"parameter": map[string]interface{}{
			"f":                 iminkResponse2.Ftoken,
			"id":                4834290508791808,
			"registrationToken": splatoonTokenResponse.Result.WebApiServerCredential.AccessToken,
			"requestId":         iminkResponse2.RequestId,
			"timestamp":         iminkResponse2.Timestamp,
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonbody: %w", err)
	}
	requestURL := "https://api-lp1.znc.srv.nintendo.net/v2/Game/GetWebServiceToken"
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Platform", "Android")
	req.Header.Set("X-ProductVersion", nsoAppVersion)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", splatoonTokenResponse.Result.WebApiServerCredential.AccessToken))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", fmt.Sprintf("com.nintendo.znca/%s(Android/7.1.2)", nsoAppVersion))
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()
	var webServiceTokenResponse webServiceApiTokenResponse
	err = json.NewDecoder(response.Body).Decode(&webServiceTokenResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json from response: %w", err)
	}
	return &webServiceTokenResponse, nil
}

func getSplatoonAccessToken(iminkResponse1 *iminkApiResponse, nintendoUserInfo *nintendoApiUserInfoResponse, nintendoTokenResponse *nintendoApiTokenResponse, nsoAppVersion string, client *http.Client) (*splatoonApiTokenResponse, error) {
	body := map[string]interface{}{
		"parameter": map[string]interface{}{
			"f":          iminkResponse1.Ftoken,
			"language":   nintendoUserInfo.Language,
			"naBirthday": nintendoUserInfo.Birthday,
			"naCountry":  nintendoUserInfo.Country,
			"naIdToken":  nintendoTokenResponse.IdToken,
			"requestId":  iminkResponse1.RequestId,
			"timestamp":  iminkResponse1.Timestamp,
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonbody: %w", err)
	}
	requestURL := "https://api-lp1.znc.srv.nintendo.net/v3/Account/Login"
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Platform", "Android")
	req.Header.Set("X-ProductVersion", nsoAppVersion)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Connection", "Keep-Alive")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("com.nintendo.znca/%s(Android/7.1.2)", nsoAppVersion))
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()
	var splatoonTokenResponse splatoonApiTokenResponse
	err = json.NewDecoder(response.Body).Decode(&splatoonTokenResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json from response: %w", err)
	}
	return &splatoonTokenResponse, nil
}

func getNintendoUserInfo(nintendoTokenResponse *nintendoApiTokenResponse, client *http.Client) (*nintendoApiUserInfoResponse, error) {
	requestURL := "https://api.accounts.nintendo.com/2.0.0/users/me"
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Host = "api.accounts.nintendo.com"
	req.Header.Set("User-Agent", "NASDKAPI; Android")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", nintendoTokenResponse.AccessToken))
	req.Header.Set("Host", "api.accounts.nintendo.com")
	req.Header.Set("Connection", "Keep-Alive")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()
	var nintendoUserInfo nintendoApiUserInfoResponse
	err = json.NewDecoder(response.Body).Decode(&nintendoUserInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json from response: %w", err)
	}
	return &nintendoUserInfo, nil
}

func getNintendoAccessToken(contestant *Contestant, client *http.Client) (*nintendoApiTokenResponse, error) {
	body := map[string]interface{}{
		"client_id":     "71b963c1b7b6d119",
		"session_token": contestant.SessionToken,
		"grant_type":    "urn:ietf:params:oauth:grant-type:jwt-bearer-session-token",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonbody: %w", err)
	}
	requestURL := "https://accounts.nintendo.com/connect/1.0.0/api/token"
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Host = "accounts.nintendo.com"
	req.Header.Set("Host", "accounts.nintendo.com")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "Keep-Alive")
	req.Header.Set("User-Agent", "Dalvik/2.1.0 (Linux; U; Android 7.1.2)")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()
	var nintendoTokenResponse nintendoApiTokenResponse
	err = json.NewDecoder(response.Body).Decode(&nintendoTokenResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json from response: %w", err)
	}
	return &nintendoTokenResponse, nil
}

func callImink(idToken string, hashMethod int, client *http.Client) (*iminkApiResponse, error) {
	body := map[string]interface{}{
		"token":       idToken,
		"hash_method": hashMethod,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonbody: %w", err)
	}
	requestURL := "https://api.imink.app/f"
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "AlternaFreshnessLeague/1.0")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()
	var iminkResponse iminkApiResponse
	err = json.NewDecoder(response.Body).Decode(&iminkResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode json from response: %w", err)
	}
	return &iminkResponse, nil
}

func getNsoAppVersion(client *http.Client) (string, error) {
	response, err := soup.GetWithClient("https://apps.apple.com/us/app/nintendo-switch-online/id1234806557", client)
	if err != nil {
		return "", fmt.Errorf("failed to get soup response: %w", err)
	}
	doc := soup.HTMLParse(response)
	versionString := doc.Find("p", "class", "whats-new__latest__version")
	return strings.ReplaceAll(versionString.Text(), "Version ", ""), nil
}

func getWebViewVersion(client *http.Client) (string, error) {
	splatnet3URL := "https://api.lp1.av5ja.srv.nintendo.net"
	response, err := soup.GetWithClient(splatnet3URL, client)
	if err != nil {
		return "", fmt.Errorf("failed to get soup response: %w", err)
	}
	doc := soup.HTMLParse(response)
	script := doc.Find("script", "defer", "defer")
	mainSrcipt := script.Attrs()["src"]
	mainJs, err := client.Get("https://api.lp1.av5ja.srv.nintendo.net" + mainSrcipt)
	if err != nil {
		return "", fmt.Errorf("failed to get mainJs: %w", err)
	}
	defer mainJs.Body.Close()
	r := regexp.MustCompile(`\b(?P<revision>[0-9a-f]{40})\b.*revision_info_not_set\"\),.*?=\"(?P<version>\d+\.\d+\.\d+)`)
	resBody, err := ioutil.ReadAll(mainJs.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read mainJs body: %w", err)
	}
	revisionAndVersion := r.FindStringSubmatch(string(resBody))
	return fmt.Sprintf("%s-%s", revisionAndVersion[2], revisionAndVersion[1][:8]), nil
}
