package messagix

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-querystring/query"

	"go.mau.fi/mautrix-meta/messagix/cookies"
	"go.mau.fi/mautrix-meta/messagix/graphql"
	"go.mau.fi/mautrix-meta/messagix/lightspeed"
	"go.mau.fi/mautrix-meta/messagix/table"
	"go.mau.fi/mautrix-meta/messagix/types"
)

func (c *Client) makeGraphQLRequest(name string, variables interface{}) (*http.Response, []byte, error) {
	graphQLDoc, ok := graphql.GraphQLDocs[name]
	if !ok {
		return nil, nil, fmt.Errorf("could not find graphql doc by the name of: %s", name)
	}

	vBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal graphql variables to json string: %v", err)
	}

	payload := c.NewHttpQuery()
	payload.FbAPICallerClass = graphQLDoc.CallerClass
	payload.FbAPIReqFriendlyName = graphQLDoc.FriendlyName
	payload.Variables = string(vBytes)
	payload.ServerTimestamps = "true"
	payload.DocID = graphQLDoc.DocId
	payload.Jssesw = graphQLDoc.Jsessw

	form, err := query.Values(payload)
	if err != nil {
		return nil, nil, err
	}

	payloadBytes := []byte(form.Encode())

	headers := c.buildHeaders(true)
	headers.Set("x-fb-friendly-name", graphQLDoc.FriendlyName)
	headers.Set("sec-fetch-dest", "empty")
	headers.Set("sec-fetch-mode", "cors")
	headers.Set("sec-fetch-site", "same-origin")
	headers.Set("origin", c.getEndpoint("base_url"))
	headers.Set("referer", c.getEndpoint("messages")+"/")

	reqUrl := c.getEndpoint("graphql")
	//c.Logger.Info().Any("url", reqUrl).Any("payload", string(payloadBytes)).Any("headers", headers).Msg("Sending graphQL request.")
	resp, respData, err := c.MakeRequest(reqUrl, "POST", headers, payloadBytes, types.FORM)
	if err == nil && resp != nil {
		cookies.UpdateFromResponse(c.cookies, resp.Header)
	}
	respData = bytes.TrimPrefix(respData, antiJSPrefix)
	return resp, respData, err
}

func (c *Client) makeLSRequest(variables *graphql.LSPlatformGraphQLLightspeedVariables, reqType int) (*table.LSTable, error) {
	strPayload, err := json.Marshal(&variables)
	if err != nil {
		return nil, err
	}

	lsVariables := &graphql.LSPlatformGraphQLLightspeedRequestPayload{
		DeviceID:              c.configs.browserConfigTable.MqttWebDeviceID.ClientID,
		IncludeChatVisibility: false,
		RequestID:             c.lsRequests,
		RequestPayload:        string(strPayload),
		RequestType:           reqType,
	}
	c.lsRequests++

	var lsRequestQueryName string
	if c.platform.IsMessenger() {
		lsRequestQueryName = "LSGraphQLRequest"
	} else {
		lsRequestQueryName = "LSGraphQLRequestIG"
	}
	_, respBody, err := c.makeGraphQLRequest(lsRequestQueryName, &lsVariables)
	if err != nil {
		return nil, err
	}

	var graphQLData *graphql.LSPlatformGraphQLLightspeedRequestQuery
	err = json.Unmarshal(respBody, &graphQLData)
	if err != nil {
		if len(respBody) < 4096 {
			c.Logger.Debug().Str("respBody", base64.StdEncoding.EncodeToString(respBody)).Msg("Errored LS response bytes")
		} else {
			c.Logger.Debug().Str("respBody", base64.StdEncoding.EncodeToString(respBody[:4096])).Msg("Errored LS response bytes (truncated)")
		}
		return nil, fmt.Errorf("failed to unmarshal LSRequest response bytes into LSPlatformGraphQLLightspeedRequestQuery struct: %v", err)
	}
	if graphQLData.ErrorCode != 0 {
		c.Logger.Warn().
			Str("error_description", graphQLData.ErrorDescription).
			Str("error_summary", graphQLData.ErrorSummary).
			Int("error_code", graphQLData.ErrorCode).
			Msg("GraphQL error in lightspeed request")
		if graphQLData.Data == nil {
			return nil, fmt.Errorf("graphql error %w", &graphQLData.ErrorResponse)
		}
	} else if graphQLData.Data == nil {
		c.Logger.Debug().RawJSON("respBody", respBody).Msg("LS response with no data and no error")
		return nil, fmt.Errorf("graphql request didn't return data")
	}
	var lightSpeedRes []byte
	var deps lightspeed.DependencyList
	if graphQLData.Data.LightspeedWebRequestForIG != nil {
		lightSpeedRes = []byte(graphQLData.Data.LightspeedWebRequestForIG.Payload)
		deps = graphQLData.Data.LightspeedWebRequestForIG.Dependencies
	} else if graphQLData.Data.Viewer.LightspeedWebRequest != nil {
		lightSpeedRes = []byte(graphQLData.Data.Viewer.LightspeedWebRequest.Payload)
		deps = graphQLData.Data.Viewer.LightspeedWebRequest.Dependencies
	} else {
		c.Logger.Debug().RawJSON("respBody", respBody).Msg("LS response with no lightspeed response data and no error")
		return nil, fmt.Errorf("graphql request didn't return LS data")
	}
	var lsData *lightspeed.LightSpeedData
	err = json.Unmarshal(lightSpeedRes, &lsData)
	if err != nil {
		c.Logger.Debug().RawJSON("respBody", respBody).Msg("Response data for errored inner response")
		return nil, fmt.Errorf("failed to unmarshal LSRequest lightspeed payload into lightspeed.LightSpeedData: %v", err)
	}

	lsTable := &table.LSTable{}
	lsDecoder := lightspeed.NewLightSpeedDecoder(deps.ToMap(), lsTable)
	lsDecoder.Decode(lsData.Steps)

	return lsTable, nil
}
