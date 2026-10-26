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
	return manager.complete(ctx, messages, 220)
}

func (manager *AIManager) complete(ctx context.Context, messages []ChatTurn, maxTokens int) (string, error) {
	payload := llamaRequest{
		Model:       manager.config.Model,
		Messages:    messages,
		Temperature: 0.35,
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
		writeError(w, http.StatusBadRequest, "Проверьте сообщение")
		return
	}
	message := strings.TrimSpace(input.Message)
	if message == "" || len([]rune(message)) > 1000 {
		writeError(w, http.StatusBadRequest, "Сообщение должно содержать от 1 до 1000 символов")
		return
	}

	ctx := r.Context()
	destination, places, hotels, err := api.store.AssistantContext(ctx, input.DestinationSlug, message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось подготовить ответ")
		return
	}

	if isAvailabilityQuestion(message) {
		response := AssistantResponse{
			Reply:       "Да, я на связи и отвечаю в реальном времени. Могу подобрать отель под ваш вайб, рассказать про тихие кофейни и собрать маршрут на день. С чего начнём?",
			Source:      "concierge",
			Hotels:      toAssistantHotels(hotels),
			Places:      places,
			Destination: destination.City,
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	if isGeneralQuestion(message) {
		reply := generalAssistantFallback()
		source := "concierge"
		if api.ai.Ready() {
			generalMessages := []ChatTurn{
				{Role: "system", Content: "Ты Odin AI — локальный консьерж сервиса бронирования отелей ODIN. Отвечай только по-русски, дружелюбно и кратко, 2–3 предложения. Расскажи, что помогаешь выбрать отель, найти локальные места и собрать маршрут. Не выдумывай факты и не обещай то, чего не можешь."},
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
			Reply:  reply,
			Source: source,
			Hotels: []AssistantHotel{},
			Places: []POI{},
		})
		return
	}

	systemPrompt := buildAssistantPrompt(destination, places, hotels)
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
	if api.ai.Ready() && !wantsHotelStay(message) {
		modelCtx, cancel := context.WithTimeout(ctx, 55*time.Second)
		generated, modelErr := api.ai.Chat(modelCtx, messages)
		cancel()
		if modelErr == nil && goodAssistantReply(generated, places, hotels, message) {
			reply = generated
			source = "llama"
		} else if modelErr != nil {
			log.Printf("AI generation fallback: %v", modelErr)
		} else {
			log.Printf("AI response rejected by grounding check")
		}
	}
	if reply == "" {
		reply = fallbackAssistantReply(destination, places, hotels, message)
	}

	response := AssistantResponse{
		Reply:       reply,
		Source:      source,
		Hotels:      toAssistantHotels(hotels),
		Places:      places,
		Destination: destination.City,
	}
	writeJSON(w, http.StatusOK, response)
}

func buildAssistantPrompt(destination Destination, places []POI, hotels []AssistantHotel) string {
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Ты Odin AI — локальный консьерж сервиса бронирования отелей ODIN. Отвечай только по-русски. ")
	contextBuilder.WriteString("Ты помогаешь выбрать отель, спланировать поездку и найти локальные места. ")
	contextBuilder.WriteString("Никогда не пиши, что «собираешь варианты» или «проверяешь доступность». Сразу давай конкретный ответ. ")
	contextBuilder.WriteString("Используй только факты из блока ДАННЫЕ. Не придумывай названия, адреса и расстояния. ")
	contextBuilder.WriteString("Называй только места и отели, которые перечислены в ДАННЫЕ. Если подходящего места нет в данных, так и скажи. ")
	contextBuilder.WriteString("Пиши дружелюбно и уверенно, простым естественным русским языком, 2–4 коротких предложения. Если уместно, назови 2–3 конкретных места или отеля. ")
	contextBuilder.WriteString("Если рекомендуешь отель, обязательно назови его цену за ночь и рейтинг из ДАННЫХ. ")
	contextBuilder.WriteString("Если вопрос не о поездке, мягко верни разговор к путешествию.\n\n")
	if destination.City != "" {
		contextBuilder.WriteString("Город: " + destination.City + "\n")
	}
	if len(places) > 0 {
		contextBuilder.WriteString("Места:\n")
		for _, place := range places {
			contextBuilder.WriteString(fmt.Sprintf("- %s (%s), %s, %s. %s\n", place.Name, place.Category, place.Address, place.Distance, place.Description))
		}
	}
	if len(hotels) > 0 {
		contextBuilder.WriteString("Отели:\n")
		for _, hotel := range hotels {
			contextBuilder.WriteString(fmt.Sprintf("- %s, %s, от %d ₽ за ночь, рейтинг %.1f, вайб: %s.\n", hotel.Name, hotel.City, hotel.PriceFrom, hotel.Rating, strings.Join(hotel.Tags, ", ")))
		}
	}
	return contextBuilder.String()
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
