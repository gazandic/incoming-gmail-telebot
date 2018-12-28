package main

import (
        "encoding/json"
        "fmt"
        "io/ioutil"
        "log"
        "net/http"
        "os"
        "time"
        "strconv"
        "encoding/base64"
        "strings"

        "golang.org/x/net/context"
        "golang.org/x/oauth2"
        "golang.org/x/oauth2/google"
        "google.golang.org/api/gmail/v1"

        "github.com/subosito/gotenv"
        "github.com/grokify/html-strip-tags-go"
	tb "gopkg.in/tucnak/telebot.v2"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
        // The file token.json stores the user's access and refresh tokens, and is
        // created automatically when the authorization flow completes for the first
        // time.
        tokFile := "token.json"
        tok, err := tokenFromFile(tokFile)
        if err != nil {
                tok = getTokenFromWeb(config)
                saveToken(tokFile, tok)
        }
        return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
        authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
        fmt.Printf("Go to the following link in your browser then type the "+
                "authorization code: \n%v\n", authURL)

        var authCode string
        if _, err := fmt.Scan(&authCode); err != nil {
                log.Fatalf("Unable to read authorization code: %v", err)
        }

        tok, err := config.Exchange(context.TODO(), authCode)
        if err != nil {
                log.Fatalf("Unable to retrieve token from web: %v", err)
        }
        return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
        f, err := os.Open(file)
        if err != nil {
                return nil, err
        }
        defer f.Close()
        tok := &oauth2.Token{}
        err = json.NewDecoder(f).Decode(tok)
        return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
        fmt.Printf("Saving credential file to: %s\n", path)
        f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
        if err != nil {
                log.Fatalf("Unable to cache oauth token: %v", err)
        }
        defer f.Close()
        json.NewEncoder(f).Encode(token)
}

func normalizeRaw(rawString string) string {
        fmt.Println("rawString")
        fmt.Println(rawString)
        rawString =  strip.StripTags(rawString)
        title := trim(rawString, ".: Summary :.", "Error detected in: Bukalapak (production)")
        summary := trim(rawString, ".: Trace :.",".: Summary :.")
        trace := trim(rawString, "Full report here:", ".: Trace :.")
        link := trim2(rawString, "Reply to this email to comment", "Full report here:")
        return "*[" + title + "]*\n" + summary + "\n ```\n" + trace + "\n```" + link
}

func trim(rawString string, right string, left string) string {
        idx := strings.Index(rawString, left) + len(left)
        idx2 := strings.Index(rawString, right)
        if idx2 == -1 {
                idx2 = len(rawString) - 1
        }
        if idx == -1 || idx2 == -1 {
                return ""
        }
        return strings.Trim(rawString[idx:idx2], "\n")
}

func trim2(rawString string, right string, left string) string {
        idx := strings.Index(rawString, left) + len(left)
        idx2 := strings.Index(rawString, right)
        if idx == -1 || idx2 == -1 {
                return ""
        }
        return "\nLink lengkap: " + strings.Trim(rawString[idx:idx2], "\n")
}

func trimStringFromDot(s string) string {
	if idx := strings.Index(s, "."); idx != -1 {
		return s[:idx]
	}
	return s
}

func getNewMessages(srv *gmail.Service, bot *tb.Bot, previous time.Time) {
        user := "me"
        rThread, err := srv.Users.Threads.Get(user, os.Getenv("THREAD_ID")).Do()
        
        if err != nil || rThread == nil {
                log.Fatalf("Unable to retrieve thread: %v", err)
        }

        id, err := strconv.Atoi(os.Getenv("YOUR_ID"))
        if err != nil {
                log.Fatalf("Unable to retrieve id on env: %v", err)
        }
        fmt.Println("Threads:")
        for _, l := range rThread.Messages {
                if l.InternalDate < (previous.Unix() * 1000) {
                        continue
                }
                rawString := l.Payload.Body.Data
                for _, p := range l.Payload.Parts {
                        b, _ := base64.StdEncoding.DecodeString(p.Body.Data)
                        rawString = rawString + string(b)
                }
                bot.Send((&tb.Chat{ ID: int64(id)}), normalizeRaw(rawString), tb.ModeMarkdown)
        }
}

func main() {
	gotenv.Load()
        previous := time.Now()
	bot, err := tb.NewBot(tb.Settings{
		Token:  os.Getenv("TELEGRAM_TOKEN"),
		Poller: &tb.LongPoller{Timeout: 20 * time.Second},
		Reporter: func(err error) {
			fmt.Println(err)
		},
	})

        b, err := ioutil.ReadFile("credentials.json")
        if err != nil {
                log.Fatalf("Unable to read client secret file: %v", err)
                return
        }

        // If modifying these scopes, delete your previously saved token.json.
        config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
        if err != nil {
                log.Fatalf("Unable to parse client secret file to config: %v", err)
        }
        client := getClient(config)

        srv, err := gmail.New(client)
        if err != nil {
                log.Fatalf("Unable to retrieve Gmail client: %v", err)
        }

        for {
                getNewMessages(srv, bot, previous)
                previous = time.Now()
                time.Sleep(120 * time.Second)
        }

}