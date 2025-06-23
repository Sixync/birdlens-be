// birdlens-be/cmd/api/ai.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	// Logic: Add the 'net/url' package to the imports.
	"net/url"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/sixync/birdlens-be/internal/response"
	"google.golang.org/api/option"
)

// --- Structs for API communication with Android Client ---

// AIIdentifyResponse mirrors the Kotlin data class expected by the Android client.
type AIIdentifyResponse struct {
	Possibilities  []string `json:"possibilities,omitempty"`
	IdentifiedBird string   `json:"identified_bird,omitempty"`
	ChatResponse   string   `json:"chat_response,omitempty"`
	ImageURL       string   `json:"image_url,omitempty"`
}

// ChatMessage mirrors the Kotlin data class for conversation history.
type ChatMessage struct {
	Role string `json:"role"` // "user" or "model"
	Text string `json:"text"`
}

// AIQuestionRequest mirrors the Kotlin data class for follow-up questions.
type AIQuestionRequest struct {
	BirdName string        `json:"bird_name"`
	Question string        `json:"question"`
	History  []ChatMessage `json:"history"`
}

// AIQuestionResponse mirrors the Kotlin data class for the answer to a follow-up question.
type AIQuestionResponse struct {
	ChatResponse string `json:"chat_response"`
}

// --- Prompts for Gemini ---

const geminiPromptIdentifyFromImage = "Identify the bird in this image. If you are very confident about one species, respond with ONLY that bird's most common English name. If it could be several similar-looking birds, respond with a comma-separated list of up to 3 most likely species. For example: 'House Sparrow, Eurasian Tree Sparrow, Chipping Sparrow'. If there is no bird in the image, respond with 'no bird'."
const geminiPromptExtractNameFromText = "You are an expert ornithologist and polyglot. Your task is to extract the bird name from the user's text.\n- If the name is specific (e.g., 'Blue Jay', 'Họa mi'), respond with ONLY that single common English name (e.g., 'Blue Jay', 'Chinese Hwamei').\n- If the name is ambiguous (e.g., 'sparrow', 'chim sẻ'), respond with a comma-separated list of up to 5 likely species in English (e.g., 'House Sparrow, Eurasian Tree Sparrow, American Tree Sparrow, Song Sparrow, Chipping Sparrow').\n- If you cannot identify a bird (e.g., the text is 'con mèo' or 'what is the weather?'), you MUST respond with the exact text: 'Error: No bird name found'.\n\nUser text: \"%s\""

// --- Handlers ---

func (app *application) identifyBirdHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	user := app.getUserFromFirebaseClaimsCtx(r)
	if user == nil {
		app.unauthorized(w, r)
		return
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(app.config.gemini.apiKey))
	if err != nil {
		app.serverError(w, r, fmt.Errorf("failed to create genai client: %w", err))
		return
	}
	defer client.Close()

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		app.badRequest(w, r, fmt.Errorf("failed to parse multipart form: %w", err))
		return
	}

	prompt := r.FormValue("prompt")
	if prompt == "" {
		app.badRequest(w, r, fmt.Errorf("prompt cannot be empty"))
		return
	}
	slog.Info("identifyBirdHandler called", "user", user.Email, "prompt", prompt)

	var parts []genai.Part

	file, _, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		imageData, readErr := io.ReadAll(file)
		if readErr != nil {
			app.serverError(w, r, fmt.Errorf("failed to read image data: %w", readErr))
			return
		}
		parts = append(parts, genai.ImageData("jpeg", imageData))
		parts = append(parts, genai.Text(geminiPromptIdentifyFromImage))
	} else {
		finalPrompt := fmt.Sprintf(geminiPromptExtractNameFromText, prompt)
		parts = append(parts, genai.Text(finalPrompt))
	}

	model := client.GenerativeModel("gemini-1.5-flash")
	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		app.serverError(w, r, fmt.Errorf("failed to generate content from Gemini: %w", err))
		return
	}

	geminiTextResponse := app.extractTextFromGeminiResponse(resp)
	if geminiTextResponse == "no bird" || strings.Contains(geminiTextResponse, "No bird name found") {
		response.JSON(w, http.StatusOK, AIIdentifyResponse{ChatResponse: "Could not identify a bird from the provided input."}, false, "Identification failed")
		return
	}

	if strings.Contains(geminiTextResponse, ",") {
		possibilities := strings.Split(geminiTextResponse, ",")
		for i, p := range possibilities {
			possibilities[i] = strings.TrimSpace(p)
		}
		response.JSON(w, http.StatusOK, AIIdentifyResponse{Possibilities: possibilities}, false, "Multiple possibilities found")
		return
	}

	identifiedBird := strings.TrimSpace(geminiTextResponse)
	slog.Info("Gemini identified a single bird", "name", identifiedBird)

	chatPrompt := fmt.Sprintf("Tell me about the %s.", identifiedBird)
	chatResp, err := model.GenerateContent(ctx, genai.Text(chatPrompt))
	if err != nil {
		app.serverError(w, r, fmt.Errorf("failed to get chat response for '%s': %w", identifiedBird, err))
		return
	}
	chatResponseText := app.extractTextFromGeminiResponse(chatResp)

	imageURL, err := app.getWikipediaImageURL(ctx, identifiedBird)
	if err != nil {
		slog.Warn("Could not fetch Wikipedia image", "bird", identifiedBird, "error", err)
	}

	finalResponse := AIIdentifyResponse{
		IdentifiedBird: identifiedBird,
		ChatResponse:   chatResponseText,
		ImageURL:       imageURL,
	}

	response.JSON(w, http.StatusOK, finalResponse, false, "Identification successful")
}

func (app *application) askAiQuestionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var req AIQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(app.config.gemini.apiKey))
	if err != nil {
		app.serverError(w, r, fmt.Errorf("failed to create genai client: %w", err))
		return
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-flash")
	chat := model.StartChat()

	chat.History = make([]*genai.Content, 0, len(req.History))
	for _, msg := range req.History {
		chat.History = append(chat.History, &genai.Content{
			Role:  msg.Role,
			Parts: []genai.Part{genai.Text(msg.Text)},
		})
	}

	resp, err := chat.SendMessage(ctx, genai.Text(req.Question))
	if err != nil {
		app.serverError(w, r, fmt.Errorf("failed to send message to Gemini: %w", err))
		return
	}

	chatResponseText := app.extractTextFromGeminiResponse(resp)
	finalResponse := AIQuestionResponse{ChatResponse: chatResponseText}

	response.JSON(w, http.StatusOK, finalResponse, false, "Question answered")
}

// --- Helper Functions ---

func (app *application) extractTextFromGeminiResponse(resp *genai.GenerateContentResponse) string {
	var b strings.Builder
	if resp != nil && len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				b.WriteString(string(txt))
			}
		}
	}
	if b.Len() == 0 {
		slog.Warn("Gemini response was empty or contained no text parts")
		return "Sorry, I could not generate a response."
	}
	return b.String()
}

type wikiQueryResponse struct {
	Query struct {
		Pages map[string]struct {
			Title     string `json:"title"`
			Thumbnail *struct {
				Source string `json:"source"`
			} `json:"thumbnail"`
		} `json:"pages"`
	} `json:"query"`
}

func (app *application) getWikipediaImageURL(ctx context.Context, title string) (string, error) {
	// Logic: This section is corrected. We now use url.Values to properly
	// build the query string for the Wikipedia API URL. This avoids the
	// nil pointer dereference error.
	params := url.Values{}
	params.Set("action", "query")
	params.Set("prop", "pageimages")
	params.Set("format", "json")
	params.Set("pithumbsize", "500")
	params.Set("redirects", "1")
	params.Set("titles", title)

	apiURL := "https://en.wikipedia.org/w/api.php?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wikipedia API returned status %d", resp.StatusCode)
	}

	var wikiResp wikiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&wikiResp); err != nil {
		return "", err
	}

	for _, page := range wikiResp.Query.Pages {
		if page.Thumbnail != nil && page.Thumbnail.Source != "" {
			return page.Thumbnail.Source, nil
		}
	}

	return "", fmt.Errorf("no image found for %s", title)
}