package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	db               *sql.DB
	mu               sync.Mutex
	waitingForNote   = make(map[int64]bool)
	waitingForDelete = make(map[int64]bool)
	token            = "TOKEN"
	dbPath           = "NAME:PASS@tcp(123.0.0.1:4567)/BD"
)

func main() {
	var err error
	db, err = sql.Open("mysql", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := createTableIfNotExists(); err != nil {
		log.Fatal(err)
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Бот успешно подключён!")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		go handleMessage(bot, update.Message)
	}
}

func createTableIfNotExists() error {
	query := `CREATE TABLE IF NOT EXISTS notes (
		id INT AUTO_INCREMENT PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		content TEXT NOT NULL
	);`
	_, err := db.Exec(query)
	return err
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	switch message.Command() {
	case "start":
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Добро пожаловать! Используйте /create для создания заметки, /view для просмотра и /delete для удаления заметки."))
	case "create":
		msg := tgbotapi.NewMessage(message.Chat.ID, "Пожалуйста, введи текст заметки.")
		bot.Send(msg)

		waitingForNote[message.Chat.ID] = true
		return
	case "view":
		displayNotes(bot, message.From.UserName, message.Chat.ID)
	case "delete":
		msg := tgbotapi.NewMessage(message.Chat.ID, "Введите порядковый номер заметки, которую вы хотите удалить. Например \"2\".")
		bot.Send(msg)

		waitingForDelete[message.Chat.ID] = true
		return
	default:
		if isWaitingForNote(message.Chat.ID) {
			content := message.Text
			mu.Lock()
			_, err := db.Exec("INSERT INTO notes (user_id, content) VALUES (?, ?)", message.From.UserName, content)
			mu.Unlock()
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Ошибка при создании заметки."))
				return
			}
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Заметка успешно создана!"))
			displayNotes(bot, message.From.UserName, message.Chat.ID)
			clearWaitingForNote(message.Chat.ID)
			return
		}

		if isWaitingForDelete(message.Chat.ID) {
			id := message.Text
			mu.Lock()
			_, err := db.Exec("DELETE FROM notes WHERE id = ? AND user_id = ?", id, message.From.UserName)
			mu.Unlock()
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Ошибка при удалении заметки. Убедитесь, что номер заметки правильный."))
				return
			}
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Заметка успешно удалена!"))
			displayNotes(bot, message.From.UserName, message.Chat.ID)
			clearWaitingForDelete(message.Chat.ID)
			return
		}
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Неизвестная команда."))
	}
}

func displayNotes(bot *tgbotapi.BotAPI, username string, chatID int64) {
	mu.Lock()
	rows, err := db.Query("SELECT id, content FROM notes WHERE user_id = ?", username)
	mu.Unlock()
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении заметок."))
		return
	}
	defer rows.Close()

	var notes []string
	for rows.Next() {
		var id int
		var content string
		if err := rows.Scan(&id, &content); err != nil {
			log.Fatal(err)
		}
		notes = append(notes, fmt.Sprintf("%d: %s", id, content))
	}

	if len(notes) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "У вас нет заметок."))
	} else {
		bot.Send(tgbotapi.NewMessage(chatID, "Ваши заметки:\n"+strings.Join(notes, "\n")))
	}
}

func isWaitingForNote(chatID int64) bool {
	return waitingForNote[chatID]
}

func clearWaitingForNote(chatID int64) {
	delete(waitingForNote, chatID)
}

func isWaitingForDelete(chatID int64) bool {
	return waitingForDelete[chatID]
}

func clearWaitingForDelete(chatID int64) {
	delete(waitingForDelete, chatID)
}
