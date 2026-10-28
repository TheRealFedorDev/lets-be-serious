package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIConfig struct {
	AutoStart bool
	BaseURL   string
	Model     string
	Server    string
	ModelPath string
	LogPath   string
	Port      int
	Threads   int
	GPULayers int
	Context   int
}

type AIManager struct {
	config AIConfig
	mu     sync.RWMutex
	cmd    *exec.Cmd
	state  string
	err    string
	client *http.Client
}

type llamaRequest struct {
	Model       string     `json:"model"`
	Messages    []ChatTurn `json:"messages"`
	Temperature float64    `json:"temperature"`
	TopP        float64    `json:"top_p"`
	MaxTokens   int        `json:"max_tokens"`
	Stream      bool       `json:"stream"`
}

type llamaResponse struct {
	Choices []struct {
		Message ChatTurn `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func LoadAIConfig() AIConfig {
	port, _ := strconvAtoi(env("ODIN_AI_PORT", "8090"), 8090)
	threads, _ := strconvAtoi(env("ODIN_AI_THREADS", "10"), 10)
	gpuLayers, _ := strconvAtoi(env("ODIN_AI_GPU_LAYERS", "99"), 99)
	contextSize, _ := strconvAtoi(env("ODIN_AI_CONTEXT", "4096"), 4096)
	baseDir := appBaseDir()
	serverPath := env("ODIN_AI_SERVER", filepath.Join(baseDir, ".ai", "llama-vulkan", "llama-server.exe"))
	modelPath := env("ODIN_AI_MODEL_PATH", filepath.Join(baseDir, ".ai", "models", "qwen2.5-3b-instruct-q4_k_m.gguf"))
	logPath := env("ODIN_AI_LOG", filepath.Join(baseDir, ".ai", "llama-server.log"))
	if absolute, err := filepath.Abs(serverPath); err == nil {
		serverPath = absolute
	}
	if absolute, err := filepath.Abs(modelPath); err == nil {
		modelPath = absolute
	}
	if absolute, err := filepath.Abs(logPath); err == nil {
		logPath = absolute
	}
	return AIConfig{
		AutoStart: envBool("ODIN_AI_AUTOSTART", true),
		BaseURL:   env("ODIN_AI_BASE_URL", fmt.Sprintf("http://127.0.0.1:%d/v1", port)),
		Model:     env("ODIN_AI_MODEL", "qwen2.5-3b"),
		Server:    serverPath,
		ModelPath: modelPath,
		LogPath:   logPath,
		Port:      port,
		Threads:   threads,
		GPULayers: gpuLayers,
		Context:   contextSize,
	}
}

func NewAIManager(config AIConfig) *AIManager {
	return &AIManager{
		config: config,
		state:  "starting",
		client: &http.Client{Timeout: 75 * time.Second},
	}
}

func (manager *AIManager) Start() {
	go manager.startAsync()
}

func (manager *AIManager) startAsync() {
	if manager.health() {
		manager.setState("warming", "")
		manager.warmup()
		return
	}
	if !manager.config.AutoStart {
		manager.setState("fallback", "autostart disabled")
		return
	}
	if _, err := os.Stat(manager.config.Server); err != nil {
		manager.setState("fallback", "local model server not found")
		log.Printf("AI fallback: server not found at %s", manager.config.Server)
		return
	}
	if _, err := os.Stat(manager.config.ModelPath); err != nil {
		manager.setState("fallback", "model file not found")
		log.Printf("AI fallback: model not found at %s", manager.config.ModelPath)
		return
	}

	if err := os.MkdirAll(filepath.Dir(manager.config.LogPath), 0o755); err != nil {
		log.Printf("AI log directory: %v", err)
	}
	logFile, err := os.OpenFile(manager.config.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("AI log file: %v", err)
	}

	args := []string{
		"-m", manager.config.ModelPath,
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", manager.config.Port),
		"-c", fmt.Sprintf("%d", manager.config.Context),
		"-t", fmt.Sprintf("%d", manager.config.Threads),
		"-ngl", fmt.Sprintf("%d", manager.config.GPULayers),
		"--no-webui",
	}
	command := exec.Command(manager.config.Server, args...)
	command.Dir = filepath.Dir(manager.config.Server)
	command.Stdout = logFile
	command.Stderr = logFile
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := command.Start(); err != nil {
		manager.setState("fallback", err.Error())
		log.Printf("AI start: %v", err)
		return
	}

	manager.mu.Lock()
	manager.cmd = command
	manager.mu.Unlock()
	log.Printf("AI model starting: %s", manager.config.ModelPath)

	manager.setState("starting", "")
	for attempt := 0; attempt < 120; attempt++ {
		time.Sleep(500 * time.Millisecond)
		if manager.health() {
			manager.setState("warming", "")
			go func() {
				_ = command.Wait()
				manager.mu.Lock()
				if manager.cmd == command {
					manager.cmd = nil
				}
				manager.mu.Unlock()
			}()
			manager.warmup()
			return
		}
	}
	manager.setState("fallback", "model start timeout")
	log.Printf("AI fallback: model start timeout")
	manager.mu.Lock()
	if manager.cmd == command {
		_ = command.Process.Kill()
		manager.cmd = nil
	}
	manager.mu.Unlock()
}

func (manager *AIManager) warmup() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := manager.complete(ctx, []ChatTurn{
		{Role: "system", Content: "Отвечай одним словом."},
		{Role: "user", Content: "Готов?"},
	}, 4)
	if err != nil {
		log.Printf("AI warmup: %v", err)
	}
	manager.setState("ready", "")
	log.Printf("AI ready: %s", manager.config.Model)
}

func (manager *AIManager) setState(state, message string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.state = state
	manager.err = message
}

func (manager *AIManager) Ready() bool {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.state == "ready"
}

func (manager *AIManager) Status() map[string]any {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return map[string]any{
		"state": manager.state,
		"ready": manager.state == "ready",
		"model": manager.config.Model,
		"error": manager.err,
	}
}

func (manager *AIManager) Close() {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.cmd != nil && manager.cmd.Process != nil {
		_ = manager.cmd.Process.Kill()
		manager.cmd = nil
	}
}

func (manager *AIManager) health() bool {
	root := strings.TrimSuffix(manager.config.BaseURL, "/v1")
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(root + "/health")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func (manager *AIManager) Chat(ctx context.Context, messages []ChatTurn) (string, error) {
	if !manager.Ready() {
		return "", errors.New("local model is not ready")
	}
	return manager.complete(ctx, messages, 360)
}

func (manager *AIManager) complete(ctx context.Context, messages []ChatTurn, maxTokens int) (string, error) {
	payload := llamaRequest{
		Model:       manager.config.Model,
		Messages:    messages,
		Temperature: 0.55,
		TopP:        0.9,
		MaxTokens:   maxTokens,
		Stream:      false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, manager.config.BaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := manager.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("model status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded llamaResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	if decoded.Error != nil {
		return "", errors.New(decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("empty model response")
	}
	return strings.TrimSpace(decoded.Choices[0].Message.Content), nil
}

func (api *API) handleAssistantStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.ai.Status())
}

func (api *API) handleAssistantChat(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Запрос отклонён")
		return
	}
	var input chatRequest
	if err := decodeJSON(w, r, &input); err != nil {
		log.Printf("assistant chat decode: %v", err)
		writeError(w, http.StatusBadRequest, "Не удалось прочитать сообщение. Обновите страницу и попробуйте снова.")
		return
	}
	message := strings.TrimSpace(input.Message)
	if message == "" || len([]rune(message)) > 1000 {
		writeError(w, http.StatusBadRequest, "Сообщение должно содержать от 1 до 1000 символов")
		return
	}

	ctx := r.Context()
	destinationSlug := strings.TrimSpace(string(input.DestinationSlug))
	hotelSlug := strings.TrimSpace(string(input.HotelSlug))
	destination, places, hotels, err := api.store.AssistantContext(ctx, destinationSlug, hotelSlug, message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось подготовить ответ")
		return
	}

	var focus *AssistantHotel
	if hotelSlug != "" && len(hotels) > 0 && hotels[0].Slug == hotelSlug {
		focused := hotels[0]
		focus = &focused
	}

	if isAvailabilityQuestion(message) {
		response := AssistantResponse{
			Reply:       "Да, я на связи и отвечаю в реальном времени. Могу подобрать отель под ваш вайб, рассказать про тихие кофейни и собрать маршрут на день. С чего начнём?",
			Source:      "concierge",
			Hotels:      toAssistantHotels(hotels),
			Places:      places,
			Destination: destination.City,
			Actions:     buildAssistantActions(destination, focus, message),
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	if focus == nil && isGeneralQuestion(message) {
		reply := generalAssistantFallback()
		source := "concierge"
		if api.ai.Ready() {
			generalMessages := []ChatTurn{
				{Role: "system", Content: "Odin AI — персональный travel-консьерж ODIN. Отвечай по-русски, тепло и конкретно, 3-5 предложений. Используй реальные данные отелей из контекста: название, город, рейтинг, цену, удобства, расстояния. Предлагай 2-4 варианта с коротким обоснованием. Если данных мало — честно скажи и предложи уточнить даты, бюджет или настроение. Не выдумывай отели и цены. В конце задай один уточняющий вопрос, чтобы продолжить диалог."},
			}
			for index, turn := range input.History {
				if index >= len(input.History)-4 {
					role := strings.ToLower(strings.TrimSpace(turn.Role))
					content := strings.TrimSpace(turn.Content)
					if (role == "user" || role == "assistant") && content != "" {
						generalMessages = append(generalMessages, ChatTurn{Role: role, Content: content})
					}
				}
			}
			generalMessages = append(generalMessages, ChatTurn{Role: "user", Content: message})
			modelCtx, cancel := context.WithTimeout(ctx, 55*time.Second)
			generated, modelErr := api.ai.Chat(modelCtx, generalMessages)
			cancel()
			if modelErr == nil && goodAssistantReply(generated, nil, nil, message) {
				reply = generated
				source = "llama"
			}
		}
		writeJSON(w, http.StatusOK, AssistantResponse{
			Reply:   reply,
			Source:  source,
			Hotels:  []AssistantHotel{},
			Places:  []POI{},
			Actions: buildAssistantActions(destination, nil, message),
		})
		return
	}

	systemPrompt := buildAssistantPrompt(destination, places, hotels, focus)
	messages := []ChatTurn{{Role: "system", Content: systemPrompt}}
	for index, turn := range input.History {
		if index >= len(input.History)-6 {
			role := strings.ToLower(strings.TrimSpace(turn.Role))
			content := strings.TrimSpace(turn.Content)
			if (role == "user" || role == "assistant") && content != "" {
				if len([]rune(content)) > 700 {
					content = string([]rune(content)[:700])
				}
				messages = append(messages, ChatTurn{Role: role, Content: content})
			}
		}
	}
	messages = append(messages, ChatTurn{Role: "user", Content: message})

	reply := ""
	source := "fallback"
	if api.ai.Ready() && (focus != nil || !wantsHotelStay(message)) {
		modelCtx, cancel := context.WithTimeout(ctx, 55*time.Second)
		generated, modelErr := api.ai.Chat(modelCtx, messages)
		cancel()
		accepted := false
		if modelErr == nil {
			if focus != nil {
				accepted = goodHotelReply(generated, focus, places)
			} else {
				accepted = goodAssistantReply(generated, places, hotels, message)
			}
		}
		if accepted {
			reply = generated
			source = "llama"
		} else if modelErr != nil {
			log.Printf("AI generation fallback: %v", modelErr)
		} else {
			log.Printf("AI response rejected by grounding check")
		}
	}
	if reply == "" {
		if focus != nil {
			reply = fallbackHotelReply(*focus, places, message)
		} else {
			reply = fallbackAssistantReply(destination, places, hotels, message)
		}
	}

	response := AssistantResponse{
		Reply:       reply,
		Source:      source,
		Hotels:      toAssistantHotels(hotels),
		Places:      places,
		Destination: destination.City,
		Focus:       focus,
		Actions:     buildAssistantActions(destination, focus, message),
	}
	writeJSON(w, http.StatusOK, response)
}

func buildAssistantPrompt(destination Destination, places []POI, hotels []AssistantHotel, focus *AssistantHotel) string {
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Ты Odin AI — локальный консьерж сервиса бронирования отелей ODIN. Отвечай только по-русски. ")
	contextBuilder.WriteString("Ты помогаешь выбрать отель, спланировать поездку и найти локальные места. ")
	contextBuilder.WriteString("Никогда не пиши, что «собираешь варианты» или «проверяешь доступность». Сразу давай конкретный ответ. ")
	contextBuilder.WriteString("Используй только факты из блока ДАННЫЕ. Не придумывай названия, адреса и расстояния. ")
	contextBuilder.WriteString("Пиши дружелюбно и уверенно, живым естественным русским языком. Не повторяй одни и те же формулировки из ответа в ответ. ")
	if focus != nil {
		contextBuilder.WriteString("Сейчас пользователь обсуждает конкретный отель. Отвечай именно про него, а не общим списком. ")
		contextBuilder.WriteString("Если спрашивают про номера — назови конкретные категории и цены. Если про цену — назови полную стоимость с налогами, сборами и завтраком. ")
		contextBuilder.WriteString("Если про расположение — используй расстояния и места рядом. Предлагай другой отель только если пользователь прямо просит альтернативу. ")
		contextBuilder.WriteString("Ответ — 2-4 предложения, в конце один уместный уточняющий вопрос.\n\n")
	} else {
		contextBuilder.WriteString("Если уместно, назови 2-3 конкретных места или отеля. Если рекомендуешь отель, обязательно назови его цену за ночь и рейтинг из ДАННЫХ. ")
		contextBuilder.WriteString("Если вопрос не о поездке, мягко верни разговор к путешествию.\n\n")
	}
	if destination.City != "" {
		contextBuilder.WriteString("Город: " + destination.City + "\n")
	}
	if focus != nil {
		writeFocusedHotel(&contextBuilder, *focus)
	}
	if len(places) > 0 {
		contextBuilder.WriteString("Места рядом:\n")
		for _, place := range places {
			contextBuilder.WriteString(fmt.Sprintf("- %s (%s), %s, %s. %s\n", place.Name, place.Category, place.Address, place.Distance, place.Description))
		}
	}
	if len(hotels) > 0 {
		label := "Отели:"
		if focus != nil {
			label = "Альтернативы в том же городе (предлагай только по запросу):"
		}
		contextBuilder.WriteString(label + "\n")
		for _, hotel := range hotels {
			if focus != nil && hotel.Slug == focus.Slug {
				continue
			}
			contextBuilder.WriteString(fmt.Sprintf("- %s, %s, от %d ₽ за ночь, рейтинг %.1f, теги: %s.\n", hotel.Name, hotel.City, hotel.PriceFrom, hotel.Rating, strings.Join(hotel.Tags, ", ")))
			if hotel.Address != "" {
				contextBuilder.WriteString(fmt.Sprintf("  Адрес: %s.\n", hotel.Address))
			}
			if hotel.ShortDescription != "" {
				contextBuilder.WriteString(fmt.Sprintf("  Описание: %s.\n", hotel.ShortDescription))
			}
			if len(hotel.Amenities) > 0 {
				contextBuilder.WriteString(fmt.Sprintf("  Удобства: %s.\n", strings.Join(hotel.Amenities, ", ")))
			}
		}
	}
	return contextBuilder.String()
}

func writeFocusedHotel(builder *strings.Builder, hotel AssistantHotel) {
	builder.WriteString("Отель в фокусе:\n")
	builder.WriteString(fmt.Sprintf("- %s, %s.\n", hotel.Name, hotel.City))
	if hotel.Stars > 0 {
		builder.WriteString(fmt.Sprintf("  Категория: %d звезды.\n", hotel.Stars))
	}
	builder.WriteString(fmt.Sprintf("  Рейтинг: %.1f из 10", hotel.Rating))
	if hotel.Reviews > 0 {
		builder.WriteString(fmt.Sprintf(" (%d отзывов)", hotel.Reviews))
	}
	builder.WriteString(".\n")
	builder.WriteString(fmt.Sprintf("  Цена: от %d ₽ за ночь", hotel.PriceFrom))
	if hotel.PriceTotal > 0 {
		builder.WriteString(fmt.Sprintf(", полная стоимость за ночь с налогами, сборами и завтраком — %d ₽", hotel.PriceTotal))
	}
	builder.WriteString(".\n")
	if hotel.Address != "" {
		builder.WriteString("  Адрес: " + hotel.Address + ".\n")
	}
	if hotel.Highlight != "" {
		builder.WriteString("  Главное: " + hotel.Highlight + ".\n")
	}
	if len(hotel.Tags) > 0 {
		builder.WriteString("  Особенности: " + strings.Join(hotel.Tags, ", ") + ".\n")
	}
	if hotel.Description != "" {
		builder.WriteString("  Описание: " + hotel.Description + "\n")
	} else if hotel.ShortDescription != "" {
		builder.WriteString("  Описание: " + hotel.ShortDescription + "\n")
	}
	if len(hotel.Amenities) > 0 {
		builder.WriteString("  Удобства: " + strings.Join(hotel.Amenities, ", ") + ".\n")
	}
	if len(hotel.Rooms) > 0 {
		parts := make([]string, 0, len(hotel.Rooms))
		for _, room := range hotel.Rooms {
			part := fmt.Sprintf("%s — %d ₽", room.Name, room.Price)
			if room.Capacity > 0 {
				part += fmt.Sprintf(", до %d гостей", room.Capacity)
			}
			if room.Bed != "" {
				part += ", " + room.Bed
			}
			if room.Size > 0 {
				part += fmt.Sprintf(", %d м²", room.Size)
			}
			parts = append(parts, part)
		}
		builder.WriteString("  Номера: " + strings.Join(parts, "; ") + ".\n")
	}
	if len(hotel.Distances) > 0 {
		builder.WriteString("  Пешая доступность: " + strings.Join(hotel.Distances, "; ") + ".\n")
	}
	if len(hotel.Nearby) > 0 {
		builder.WriteString("  Рядом: " + strings.Join(hotel.Nearby, "; ") + ".\n")
	}
}

func goodHotelReply(reply string, focus *AssistantHotel, places []POI) bool {
	normalized := strings.ToLower(strings.TrimSpace(reply))
	if len([]rune(normalized)) < 45 {
		return false
	}
	badPatterns := []string{"собираю вариант", "проверяю доступ", "не могу ответить", "как языковая модель", "я не имею доступа", "в данных отелях"}
	for _, pattern := range badPatterns {
		if strings.Contains(normalized, pattern) {
			return false
		}
	}
	name := strings.ToLower(focus.Name)
	if strings.Contains(normalized, name) {
		return true
	}
	if strings.Contains(normalized, "₽") || strings.Contains(normalized, "рейтинг") {
		return true
	}
	for _, place := range places {
		if place.Name != "" && strings.Contains(normalized, strings.ToLower(place.Name)) {
			return true
		}
	}
	return false
}

func fallbackHotelReply(hotel AssistantHotel, places []POI, message string) string {
	lower := strings.ToLower(message)

	if containsAny(lower, "номер", "комнат", "размещ", "люкс", "сьют", "suite", "studio") {
		if len(hotel.Rooms) > 0 {
			var builder strings.Builder
			builder.WriteString(fmt.Sprintf("В %s %d %s на выбор. ", hotel.Name, len(hotel.Rooms), pluralWord(len(hotel.Rooms), "категория", "категории", "категорий")))
			for index, room := range hotel.Rooms {
				if index >= 3 {
					break
				}
				if index > 0 {
					builder.WriteString("; ")
				}
				builder.WriteString(fmt.Sprintf("«%s» — %d ₽ за ночь", room.Name, room.Price))
				if room.Capacity > 0 {
					builder.WriteString(fmt.Sprintf(", до %d гостей", room.Capacity))
				}
				if room.Size > 0 {
					builder.WriteString(fmt.Sprintf(", %d м²", room.Size))
				}
			}
			builder.WriteString(fmt.Sprintf(". С налогами, сборами и завтраком полная стоимость стартует от %d ₽. Какой формат ближе — компактный или с гостиной?", hotel.PriceTotal))
			return builder.String()
		}
	}

	if containsAny(lower, "цен", "стоит", "стоимость", "бюджет", "дорого", "дешев", "завтрак", "налог", "сбор") {
		if hotel.PriceTotal > 0 {
			return fmt.Sprintf("Номер в %s стартует от %d ₽ за ночь. С налогами, сервисным сбором и завтраком итог — %d ₽ за ночь, без сюрпризов на кассе. Если скажете, сколько гостей и на какие даты, подскажу точнее — или помогу выбрать номер.", hotel.Name, hotel.PriceFrom, hotel.PriceTotal)
		}
		return fmt.Sprintf("Номер в %s стартует от %d ₽ за ночь, итоговую стоимость с налогами и завтраком покажу в карточке отеля. На какие даты и сколько гостей планируете?", hotel.Name, hotel.PriceFrom)
	}

	if containsAny(lower, "рядом", "располож", "далеко", "метро", "пляж", "парк", "погул", "экскурс", "посмотреть", "куда сходить", "прогул") {
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("%s находится по адресу %s. ", hotel.Name, hotel.Address))
		if len(hotel.Distances) > 0 {
			builder.WriteString("Пешком: " + strings.Join(hotel.Distances, ", ") + ". ")
		}
		if len(hotel.Nearby) > 0 {
			builder.WriteString("Рядом: " + strings.Join(hotel.Nearby, "; ") + ". ")
		}
		if len(places) > 0 {
			names := make([]string, 0, len(places))
			for _, place := range places {
				names = append(names, place.Name)
			}
			builder.WriteString("Из проверенного советую " + strings.Join(names, ", ") + ". ")
		}
		builder.WriteString("Хотите, соберу маршрут на день вокруг отеля?")
		return builder.String()
	}

	if containsAny(lower, "спа", "бассейн", "саун", "отдых", "расслаб", "аквапарк") {
		features := make([]string, 0)
		for _, amenity := range hotel.Amenities {
			if containsAny(strings.ToLower(amenity), "спа", "бассейн", "саун", "хаммам", "джакузи") {
				features = append(features, amenity)
			}
		}
		if len(features) > 0 {
			return fmt.Sprintf("Для восстановления в %s есть %s. После прогулки удобно вернуться и провести вечер без спешки. Хотите, подскажу, какие номера ближе к спа-зоне?", hotel.Name, strings.Join(features, ", "))
		}
		return fmt.Sprintf("В %s нет большого спа-комплекса, зато есть спокойная атмосфера и удобства для отдыха: %s. Могу предложить отель с бассейном и спа в этом же городе, если это важно.", hotel.Name, strings.Join(hotel.Amenities, ", "))
	}

	if containsAny(lower, "парков", "машин", "авто", "трансфер", "аэропорт") {
		hasParking := false
		for _, amenity := range hotel.Amenities {
			if containsAny(strings.ToLower(amenity), "парков", "трансфер") {
				hasParking = true
				break
			}
		}
		if hasParking {
			return fmt.Sprintf("Да, у %s есть парковка и/или трансфер — это указано в удобствах отеля. Адрес для навигатора: %s. Уточните, нужна ли парковка на всё время проживания, и я подскажу детали.", hotel.Name, hotel.Address)
		}
		return fmt.Sprintf("В удобствах %s парковка не заявлена. Ближайшие варианты с парковкой могу предложить в этом же городе — скажите, на каком транспорте планируете приехать?", hotel.Name)
	}

	if containsAny(lower, "животн", "с собак", "с кошк", "питом") {
		for _, tag := range hotel.Tags {
			if strings.Contains(strings.ToLower(tag), "животн") {
				return fmt.Sprintf("Да, %s принимает гостей с животными — это отмечено в особенностях отеля. Уточните, пожалуйста, вес питомца: помогу проверить условия размещения.", hotel.Name)
			}
		}
		return fmt.Sprintf("В данных %s размещение с животными не отмечено. Могу подобрать отель в этом же городе, где питомцев принимают официально. Какой у вас питомец?", hotel.Name)
	}

	// Default: a warm, specific summary that changes depending on what the hotel offers.
	intro := "Отличный выбор"
	switch len([]rune(hotel.Name)) % 3 {
	case 1:
		intro = "Хороший вариант"
	case 2:
		intro = "С удовольствием расскажу"
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s: %s — %s, рейтинг %.1f", intro, hotel.Name, hotel.City, hotel.Rating))
	if hotel.Reviews > 0 {
		builder.WriteString(fmt.Sprintf(" по %d отзывам", hotel.Reviews))
	}
	builder.WriteString(". ")
	if hotel.ShortDescription != "" {
		builder.WriteString(hotel.ShortDescription + " ")
	}
	if len(hotel.Amenities) > 0 {
		builder.WriteString("Есть " + strings.Join(hotel.Amenities, ", ") + ". ")
	}
	builder.WriteString(fmt.Sprintf("Номер стартует от %d ₽, с налогами и завтраком — %d ₽ за ночь. ", hotel.PriceFrom, hotel.PriceTotal))
	if len(hotel.Nearby) > 0 {
		builder.WriteString("Рядом: " + strings.Join(hotel.Nearby, "; ") + ". ")
	}
	builder.WriteString("Что для вас важнее — тишина, вид из номера или завтраки?")
	return builder.String()
}

func pluralWord(count int, one, few, many string) string {
	mod10 := count % 10
	mod100 := count % 100
	if mod10 == 1 && mod100 != 11 {
		return one
	}
	if mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20) {
		return few
	}
	return many
}

func goodAssistantReply(reply string, places []POI, hotels []AssistantHotel, message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(reply))
	if len([]rune(normalized)) < 60 {
		return false
	}
	badPatterns := []string{"собираю вариант", "проверяю доступ", "не могу ответить", "как языковая модель", "я не имею доступа"}
	for _, pattern := range badPatterns {
		if strings.Contains(normalized, pattern) {
			return false
		}
	}

	checkPlaces := !wantsHotelStay(message) && len(places) > 0
	if checkPlaces {
		matchedPlaces := 0
		for _, place := range places {
			if strings.Contains(normalized, strings.ToLower(place.Name)) {
				matchedPlaces++
			}
		}
		required := 1
		if len(places) >= 2 {
			required = 2
		}
		return matchedPlaces >= required
	}
	if len(hotels) > 0 {
		hasSpecifics := strings.Contains(normalized, "₽") || strings.Contains(normalized, "рейтинг") || strings.Contains(normalized, "ночь")
		if wantsHotelStay(message) && !hasSpecifics {
			return false
		}
		for _, hotel := range hotels {
			if strings.Contains(normalized, strings.ToLower(hotel.Name)) {
				return true
			}
		}
		return false
	}
	return true
}

func wantsHotelStay(message string) bool {
	lower := strings.ToLower(message)
	return containsAny(lower, "отель", "отел", "номер", "заброни", "где останов", "поездк", "жиль", "апартамент")
}

func fallbackAssistantReply(destination Destination, places []POI, hotels []AssistantHotel, message string) string {
	if isAvailabilityQuestion(message) {
		return "Да, я на связи и отвечаю сразу. Могу подобрать отель под ваш вайб, найти тихую кофейню или собрать маршрут на день. С чего начнём?"
	}

	city := destination.City
	if city == "" && len(hotels) > 0 {
		city = hotels[0].City
	}
	if city == "" {
		city = "выбранном городе"
	}

	if wantsHotelStay(message) && len(hotels) > 0 {
		var builder strings.Builder
		builder.WriteString("Вот отели " + cityPhrase(city) + ", на которые стоит посмотреть:")
		for index, hotel := range hotels {
			if index >= 3 {
				break
			}
			builder.WriteString(fmt.Sprintf("\n• %s — рейтинг %.1f, от %d ₽ за ночь, вайб: %s.", hotel.Name, hotel.Rating, hotel.PriceFrom, strings.Join(hotel.Tags, ", ")))
		}
		builder.WriteString("\nЕсли скажете, что важнее — тишина, море или завтраки, — сузим выбор.")
		return builder.String()
	}

	if len(places) > 0 {
		var builder strings.Builder
		switch places[0].Category {
		case "coffee":
			builder.WriteString("Да, я на связи. " + capitalizeFirst(cityPhrase(city)) + " без толпы советую:")
		case "food":
			builder.WriteString(capitalizeFirst(cityPhrase(city)) + " стоит зайти:")
		case "walk":
			builder.WriteString("Для прогулки " + cityPhrase(city) + " отличный вариант:")
		case "spa":
			builder.WriteString("Для спокойного вечера " + cityPhrase(city) + " подойдёт:")
		default:
			builder.WriteString(capitalizeFirst(cityPhrase(city)) + " обратите внимание на:")
		}
		for index, place := range places {
			if index >= 3 {
				break
			}
			builder.WriteString(fmt.Sprintf("\n• %s — %s, %s.", place.Name, place.Address, place.Distance))
		}
		if len(hotels) > 0 {
			builder.WriteString("\nЕсли нужен отель рядом, посмотрите " + hotels[0].Name + " — " + fmt.Sprintf("%.1f", hotels[0].Rating) + " и от " + fmt.Sprintf("%d", hotels[0].PriceFrom) + " ₽ за ночь.")
		}
		return builder.String()
	}

	if len(hotels) > 0 {
		var builder strings.Builder
		builder.WriteString("Могу сразу предложить отели " + cityPhrase(city) + ":")
		for index, hotel := range hotels {
			if index >= 3 {
				break
			}
			builder.WriteString(fmt.Sprintf("\n• %s — рейтинг %.1f, от %d ₽ за ночь.", hotel.Name, hotel.Rating, hotel.PriceFrom))
		}
		builder.WriteString("\nСкажите, что важнее: тишина, вид или завтраки — сузим выбор.")
		return builder.String()
	}

	return "Я на связи. Опишите поездку одним предложением — например, «тихий отель у моря для двоих» или «где пить кофе в Москве». Сразу предложу конкретные варианты."
}

func isAvailabilityQuestion(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "ты работаешь") ||
		strings.Contains(lower, "работаешь?") ||
		strings.Contains(lower, "ты тут") ||
		strings.Contains(lower, "ты на связи")
}

func isGeneralQuestion(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return containsAny(
		lower,
		"что ты умеешь",
		"что умеешь",
		"расскажи о себе",
		"кто ты",
		"привет",
		"здравствуй",
		"добрый день",
		"спасибо",
	)
}

func generalAssistantFallback() string {
	return "Я Odin AI — локальный консьерж ODIN. Помогу выбрать отель под ваш вайб, найти тихое место для кофе или ужина и собрать маршрут на день. Просто опишите поездку — например, «тихий отель у моря для двоих»."
}

func cityPhrase(city string) string {
	switch city {
	case "Москва":
		return "в Москве"
	case "Санкт-Петербург":
		return "в Санкт-Петербурге"
	case "Сочи":
		return "в Сочи"
	case "Казань":
		return "в Казани"
	case "Калининград":
		return "в Калининграде"
	case "Алтай":
		return "на Алтае"
	case "выбранном городе":
		return "в выбранном городе"
	default:
		return "в " + city
	}
}

func capitalizeFirst(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func toAssistantHotels(hotels []AssistantHotel) []AssistantHotel {
	if hotels == nil {
		return []AssistantHotel{}
	}
	return hotels
}

func strconvAtoi(value string, fallback int) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback, err
	}
	return parsed, nil
}
