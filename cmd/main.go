package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"gobot/bootstrap"
	"gobot/db"
)

func main() {
	// Set log
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	env := bootstrap.NewEnv()

	database := connectToDB(env)

	DBName := env.DBDriver
	runDBMigration(database, DBName, env.MigrationURL)

	store := db.NewStore(database)
	telegramBot(env, store)
}

func connectToDB(env *bootstrap.Env) *sql.DB {
	log.Print(env.DBSource)
	database, err := sql.Open(env.DBDriver, env.DBSource)
	if err != nil {
		log.Fatalf("failed to connect to Postgresql: %v", err)
	}
	err = database.Ping()
	if err != nil {
		log.Fatalf("failed to connect to Postgresql: %v", err)
	}
	log.Print("connected to Postgresql")

	return database
}

func runDBMigration(db *sql.DB, DBname, migrationURL string) {
	driver, _ := postgres.WithInstance(db, &postgres.Config{})

	migration, err := migrate.NewWithDatabaseInstance(
		migrationURL,
		DBname, // "postgres"
		driver)
	if err != nil {
		log.Fatalf("cannot create new migrate instance: %v", err)
	}

	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("failed to run migrate up: %v", err)
	}

	log.Printf("db migrated successfully")
}

func telegramBot(env *bootstrap.Env, store db.Store) {
    bot := createTelegramBot(env)
    log.Printf("Authorized on account %s", bot.Self.UserName)

    updates := getUpdates(bot)
    for update := range updates {
        if update.Message == nil {
            continue
        }
        log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
        handleCommand(bot, update, env, store)
    }
}

func createTelegramBot(env *bootstrap.Env) *tgbotapi.BotAPI {
    bot, err := tgbotapi.NewBotAPI(env.BotToken)
    if err != nil {
        log.Fatal(err)
    }
    return bot
}

func getUpdates(bot *tgbotapi.BotAPI) tgbotapi.UpdatesChannel {
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    updates, err := bot.GetUpdatesChan(u)
    if err != nil {
        log.Fatal(err)
    }
    return updates
}

func handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, env *bootstrap.Env, store db.Store) {
    switch update.Message.Command() {
    case "start":
        handleStartCommand(bot, update)
    case "information":
        handleInformationCommand(bot, update, env, store)
    case "statistics":
        handleStatisticsCommand(bot, update, env, store)
    default:
        handleUnknownCommand(bot, update)
    }
}

func handleStartCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
    sendMessage(bot, update.Message.Chat.ID, "Привет! Я бот, который показывает курс криптовалюты в долларах. Чтобы узнать курс, отправьте мне команду /information и название криптовалюты. Например, /information BTC.")
}

func handleInformationCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, env *bootstrap.Env, store db.Store) {
    args := update.Message.CommandArguments()
    if args == "" {
        sendMessage(bot, update.Message.Chat.ID, "Вы не указали криптовалюту. Чтобы узнать курс, отправьте мне команду /information и название криптовалюты. Например, /information BTC.")
        return
    }

    cryptocurrency := args
    rate, err := getExchangeRate(env, cryptocurrency)
    if err != nil {
        sendMessage(bot, update.Message.Chat.ID, "Не удалось получить курс криптовалюты.")
        return
    }

	err = store.SaveRequest(update.Message.From.ID, cryptocurrency)
    if  err != nil {
        sendMessage(bot, update.Message.Chat.ID, "Не удалось сохранить ваш запрос.")
    }

    sendMessage(bot, update.Message.Chat.ID, formatCourse(cryptocurrency, rate))
}

func getExchangeRate(env *bootstrap.Env, cryptocurrency string) (float64, error) {
	// Getting the cryptocurrency rate from the API
	url := fmt.Sprintf("http://api.coinlayer.com/live?access_key=%s&symbols=%s",
		env.APIKey, cryptocurrency)
	res, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return 0, fmt.Errorf("failed to get a cryptocurrency exchange rate. %v", err)
	}
	defer res.Body.Close()

	// Parsing JSON resonse
	var exchangeRates struct {
		Base  string             `json:"base"`
		Rates map[string]float64 `json:"rates"`
	}
	err = json.NewDecoder(res.Body).Decode(&exchangeRates)
	if err != nil {
		log.Println(err)
		return 0, fmt.Errorf("failed to parse the response from the API. %v", err)
	}

	// Get exchange rate
	rate := exchangeRates.Rates[cryptocurrency]
	return rate, nil
}

func formatCourse(cryptocurrency string, rate float64) string {
	return fmt.Sprintf("Курс %s составляет %f USD.", cryptocurrency, rate)
}

func handleStatisticsCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, env *bootstrap.Env, store db.Store) {
	count, err := store.CountRequests(update.Message.From.ID)
	if err != nil {
		log.Println(err)
		sendMessage(bot, update.Message.Chat.ID, "Не удалось получить статистику.")
		return
	}

	firstRequest, err := store.GetFirstRequestTime(update.Message.From.ID)
	if err != nil {
		log.Println(err)
		sendMessage(bot, update.Message.Chat.ID, "Не удалось получить статистику.")
		return
	}

	statisticsText := formatStatistics(count, firstRequest)
	sendMessage(bot, update.Message.Chat.ID, statisticsText)
}

func formatStatistics(count int, firstRequest time.Time) string {
	return fmt.Sprintf("Вы сделали %d запросов. Первый запрос был %s.", count, firstRequest.Format("07.03.2023 18:04:05"))
}

func handleUnknownCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	sendMessage(bot, update.Message.Chat.ID, "Неизвестная команда. Чтобы узнать курс криптовалюты, отправьте мне команду /information и название криптовалюты. Например, /information BTC.")
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}
