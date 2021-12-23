package twilio

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/shared"
	"github.com/twilio/twilio-go"
	twilioUtil "github.com/twilio/twilio-go/client"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

var logg = logger.NewLogger()

type ClientWrapper struct {
	client           *twilio.RestClient
	config           shared.TwilioConfig
	requestValidator twilioUtil.RequestValidator
	webhookBaseURL   string
	devMode          bool
}

func NewClient(config shared.TwilioConfig, appUrl string, devMode bool) *ClientWrapper {
	client := twilio.NewRestClientWithParams(twilio.RestClientParams{
		Username: config.AccountSid,
		Password: config.AuthToken,
	})

	return &ClientWrapper{
		client:           client,
		config:           config,
		devMode:          devMode,
		webhookBaseURL:   appUrl,
		requestValidator: twilioUtil.NewRequestValidator(config.AuthToken),
	}
}

func (cw *ClientWrapper) SendMessage(to, msg string) error {
	params := &openapi.CreateMessageParams{}
	params.SetMessagingServiceSid(cw.config.MessagingServiceSid)
	params.SetTo(to)
	params.SetBody(msg)

	// Only log messages to stdout in devMode
	if cw.devMode {
		logg.Infof(fmt.Sprintf("%v to: %v; body: %v", colors.Green("[message]"), to, msg))
		return nil
	}

	resp, err := cw.client.ApiV2010.CreateMessage(params)
	if err != nil {
		return err
	}

	if resp.ErrorMessage != nil {
		return fmt.Errorf(*resp.ErrorMessage)
	}

	logg.Infof(fmt.Sprintf("%v new mesasge sent!", colors.Green("[message]")))

	return nil
}

func (cw *ClientWrapper) ValidateRequest(path string, urlValues url.Values, expectedSignature string) bool {
	// Get 'urlValues' as map[string]string so it's compatible with twilio request validator
	params := make(map[string]string)
	for key, val := range urlValues {
		params[key] = strings.Join(val, ",")
	}

	return cw.requestValidator.Validate(fullRequestURL(cw.webhookBaseURL, path), params, expectedSignature)
}

func fullRequestURL(appUrl, path string) string {
	refinedUrl := strings.TrimSuffix(appUrl, "/")

	// Set default scheme to https
	if !strings.HasPrefix(refinedUrl, "http") {
		refinedUrl = refinedUrl + "https://"
	}

	return refinedUrl + path
}
