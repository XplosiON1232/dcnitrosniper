package sniper

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sniper/global"
	"sniper/logger"
	"sniper/request"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	claimedDescriptions = []string{
		"What competition ",
		"Another One! :smiling_face_with_3_hearts:",
		"Ah shit, here we go again :rolling_eyes:",
		"I'm on fire ",
		"I bet he's sorry he sent that ",
	}

	missedDescriptions = []string{
		"I was on the toilet ",
		"Sorry, I accidentally fell asleep :sleeping:",
		"I'm sorry, I'll do better next time :heart_hands:",
		"Don't give up on me, I won't let you down :pensive:",
		"I'm just warming up :index_pointing_at_the_viewer:",
		"Let's not talk about this :rolling_eyes:",
		"Don't worry, be happy :smile:",
		"I'm in my mom's car, vroom vroom :red_car:",
	}
)

func getRandomClaimedDescription() string {
	return claimedDescriptions[rand.Intn(len(claimedDescriptions))]
}

func getRandomMissedDescription() string {
	return missedDescriptions[rand.Intn(len(missedDescriptions))]
}

func GetDiscordBuildNumber() (int, error) {
	makeGetReq := func(urlStr string) ([]byte, error) {
		ReqUrl, err := url.Parse(strings.TrimSpace(urlStr))
		if err != nil {
			return nil, err
		}

		client := &http.Client{
			Timeout: time.Duration(10 * time.Second),
			Transport: &http.Transport{
				DisableKeepAlives: true,
				IdleConnTimeout:   0,
			},
		}

		res, err := client.Get(ReqUrl.String())
		if err != nil {
			return nil, err
		}

		defer res.Body.Close()

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		client.CloseIdleConnections()
		return bodyBytes, nil
	}

	// Try different Discord URLs
	urls := []string{
		"https://discord.com/app",
		"https://discord.com",
		"https://discord.com/channels/@me",
	}

	var lastErr error
	for _, urlStr := range urls {
		responeBody, err := makeGetReq(urlStr)
		if err != nil {
			lastErr = err
			continue
		}

		// Try finding the build number directly in the HTML
		buildMatches := regexp.MustCompile(`"buildNumber":"(\d+)"`).FindStringSubmatch(string(responeBody))
		if len(buildMatches) > 1 {
			client_build_number, err := strconv.Atoi(buildMatches[1])
			if err != nil {
				lastErr = err
				continue
			}
			return client_build_number, nil
		}

		// Try finding the build number in a different format
		buildMatches = regexp.MustCompile(`BUILD_NUMBER:\s*"(\d+)"`).FindStringSubmatch(string(responeBody))
		if len(buildMatches) > 1 {
			client_build_number, err := strconv.Atoi(buildMatches[1])
			if err != nil {
				lastErr = err
				continue
			}
			return client_build_number, nil
		}

		// Look for script files
		scriptFiles := regexp.MustCompile(`<script\s+[^>]*src="([^"]+)"`).FindAllStringSubmatch(string(responeBody), -1)
		for _, match := range scriptFiles {
			if len(match) < 2 {
				continue
			}
			
			scriptUrl := match[1]
			if !strings.HasPrefix(scriptUrl, "http") {
				if !strings.HasPrefix(scriptUrl, "/") {
					scriptUrl = "/" + scriptUrl
				}
				scriptUrl = "https://discord.com" + scriptUrl
			}

			responeBody, err := makeGetReq(scriptUrl)
			if err != nil {
				continue
			}

			// Try to find the build number in the JS file
			buildMatches := regexp.MustCompile(`"buildNumber"\s*:\s*"?(\d{6})"?`).FindStringSubmatch(string(responeBody))
			if len(buildMatches) > 1 {
				client_build_number, err := strconv.Atoi(buildMatches[1])
				if err != nil {
					continue
				}
				return client_build_number, nil
			}

			// Try the old format
			buildNumberMatches := regexp.MustCompile(`"[0-9]{6}"`).FindAllString(string(responeBody), -1)
			if len(buildNumberMatches) > 0 {
				client_build_number_str := strings.Replace(buildNumberMatches[0], "\"", "", -1)
				client_build_number, err := strconv.Atoi(client_build_number_str)
				if err != nil {
					continue
				}
				return client_build_number, nil
			}
		}
	}

	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("could not find Discord build number in any location")
}

type GiftData struct {
	GotData    bool
	StatusCode int
	Body       string
	End        time.Time
}

func CheckGiftLink(code string) (giftData GiftData) {
	var err error = nil
	giftData.StatusCode, giftData.Body, giftData.End, err = request.ClaimCode(code)
	giftData.GotData = (err == nil)
	if err != nil {
		fmt.Println(err)
	}

	/*
		if strings.Contains(giftData.Body, "Unknown Gift Code") {
			giftData.Status = UnknownGift
			return
		}

		if strings.Contains(strings.ToLower(giftData.Body), "subscription_plan") {
			giftData.Status = Claimed

			if reNitroType.Match([]byte(giftData.Body)) {
				giftData.NitroType = reNitroType.FindStringSubmatch(giftData.Body)[1]
			}

			return
		}

		giftData.Status = Unclaimed
	*/

	return
}

type embedFieldStruct struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type EmbedStruct struct {
	Color       int                `json:"color"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Timestamp   time.Time          `json:"timestamp,omitempty"`
	Fields      []embedFieldStruct `json:"fields"`
	Thumbnail   struct {
		URL string `json:"url,omitempty"`
	} `json:"thumbnail"`
	Footer struct {
		Text    string `json:"text"`
		IconUrl string `json:"icon_url,omitempty"`
	} `json:"footer"`
}

type WebhookData struct {
	Content interface{}   `json:"content"`
	Embeds  []EmbedStruct `json:"embeds"`
}

func isImageURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	ext := filepath.Ext(parsedURL.Path)
	ext = strings.ToLower(ext)

	// Check if the file extension corresponds to an embeddable image type
	switch ext {
	case ".png", ".gif", ".jpg", ".jpeg", ".bmp", ".tiff", ".webp", ".svg", ".ico":
		return true
	default:
		return false
	}
}

func WebhookSuccess(Code string, Delay time.Duration, Sniper, Type, Sender, GuildID, GuildName string) {
	if global.Config.Discord.Webhooks.Successful == "" {
		return
	}

	embedMedia := "https://imgur.com/a/d1ZW6aX"
	if len(global.Config.Discord.Webhooks.EmbedMedia) > 0 && isImageURL(global.Config.Discord.Webhooks.EmbedMedia) {
		embedMedia = global.Config.Discord.Webhooks.EmbedMedia
	}

	// YYYY-MM-DDTHH:MM:SS.MSSZ
	//timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")

	data := WebhookData{}
	data.Content = nil

	embedData := EmbedStruct{}
	embedData.Color = 7293676
	embedData.Title = "Unknown Sniper"
	embedData.Description = ":white_check_mark: | " + getRandomClaimedDescription()
	embedData.Timestamp = time.Now()
	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Code",
		Value:  "`" + Code + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Delay",
		Value:  "`" + fmt.Sprintf("%f", Delay.Seconds()) + "s`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Type",
		Value:  "`" + Type + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Sniper",
		Value:  "`" + Sniper + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Sender",
		Value:  "`" + Sender + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Guild",
		Value:  "`" + GuildID + " | " + GuildName + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Claimer",
		Value:  "`" + Sniper + "`",
		Inline: true,
	})

	embedData.Thumbnail.URL = embedMedia

	embedData.Footer.Text = global.Hostname + " | " + global.SnipingToken[len(global.SnipingToken)-5:]
	embedData.Footer.IconUrl = embedMedia

	data.Embeds = append(data.Embeds, embedData)

	body, _ := json.Marshal(data)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.Header.SetContentType("application/json")
	req.SetBody(body)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI(global.Config.Discord.Webhooks.Successful)
	req.SetTimeout(time.Minute)

	if err := fasthttp.Do(req, res); err != nil {
		logger.Error("Failed to send webhook (success)", logger.FieldAny("error", err))
		return
	}
}

func WebhookFail(Code string, Delay time.Duration, Sniper, Sender, GuildID, GuildName, Response string) {
	if global.Config.Discord.Webhooks.Missed == "" {
		return
	}

	embedMedia := "https://imgur.com/a/d1ZW6aX"
	if len(global.Config.Discord.Webhooks.EmbedMedia) > 0 && isImageURL(global.Config.Discord.Webhooks.EmbedMedia) {
		embedMedia = global.Config.Discord.Webhooks.EmbedMedia
	}

	// YYYY-MM-DDTHH:MM:SS.MSSZ
	//timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")

	data := WebhookData{}
	data.Content = nil

	embedData := EmbedStruct{}
	embedData.Color = 7293676
	embedData.Title = "Unknown Sniper"
	embedData.Description = ":x: | " + getRandomMissedDescription()
	embedData.Timestamp = time.Now()
	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Code",
		Value:  "`" + Code + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Delay",
		Value:  "`" + fmt.Sprintf("%f", Delay.Seconds()) + "s`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Sniper",
		Value:  "`" + Sniper + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Sender",
		Value:  "`" + Sender + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Guild",
		Value:  "`" + GuildID + " | " + GuildName + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Claimer",
		Value:  "`" + Sniper + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Response",
		Value:  "```" + Response + "```",
		Inline: true,
	})

	embedData.Thumbnail.URL = embedMedia

	embedData.Footer.Text = global.Hostname + " | " + global.SnipingToken[len(global.SnipingToken)-5:]
	embedData.Footer.IconUrl = embedMedia

	data.Embeds = append(data.Embeds, embedData)

	body, _ := json.Marshal(data)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.Header.SetContentType("application/json")
	req.SetBody(body)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI(global.Config.Discord.Webhooks.Missed)
	req.SetTimeout(time.Minute)

	if err := fasthttp.Do(req, res); err != nil {
		logger.Error("Failed to send webhook (miss)", logger.FieldAny("error", err))
		return
	}
}

func WebhookUpdate(oldVersion, newVersion string) {
	if global.Config.Discord.Webhooks.Updates == "" {
		return
	}

	embedMedia := "https://i.imgur.com/AqjEQ3j.gif"
	if len(global.Config.Discord.Webhooks.EmbedMedia) > 0 && isImageURL(global.Config.Discord.Webhooks.EmbedMedia) {
		embedMedia = global.Config.Discord.Webhooks.EmbedMedia
	}

	// YYYY-MM-DDTHH:MM:SS.MSSZ
	//timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")

	data := WebhookData{}
	data.Content = nil

	embedData := EmbedStruct{}
	embedData.Color = 7293676
	embedData.Title = "Unknown Sniper"
	embedData.Description = "New Sniper Update Released!\nPlease update."
	embedData.Timestamp = time.Now()
	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "Old Version",
		Value:  "`" + oldVersion + "`",
		Inline: true,
	})

	embedData.Fields = append(embedData.Fields, embedFieldStruct{
		Name:   "New Version",
		Value:  "`" + newVersion + "`",
		Inline: true,
	})

	embedData.Thumbnail.URL = embedMedia

	embedData.Footer.Text = global.Hostname
	embedData.Footer.IconUrl = embedMedia

	data.Content = "@everyone"
	data.Embeds = append(data.Embeds, embedData)

	body, _ := json.Marshal(data)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.Header.SetContentType("application/json")
	req.SetBody(body)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI(global.Config.Discord.Webhooks.Updates)
	req.SetTimeout(time.Minute)

	if err := fasthttp.Do(req, res); err != nil {
		logger.Error("Failed to send webhook (update)", logger.FieldAny("error", err))
		return
	}
}
