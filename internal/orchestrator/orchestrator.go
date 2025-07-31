package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tools"
)

// ModelSize represents t-shirt sizes for models.
type ModelSize string

const (
	SizeXS ModelSize = "XS" // < 2B params (super fast)
	SizeS  ModelSize = "S"  // 2-4B params (fast)
	SizeM  ModelSize = "M"  // 7-13B params (balanced)
	SizeL  ModelSize = "L"  // 14-34B params (powerful)
	SizeXL ModelSize = "XL" // 70B+ params (maximum power)
)

// TaskType defines what kind of task we're doing.
type TaskType string

const (
	TaskSummarize TaskType = "summarize"
	TaskAnalyze   TaskType = "analyze"
	TaskCode      TaskType = "code"
	TaskEdit      TaskType = "edit"
	TaskReview    TaskType = "review"
	TaskChat      TaskType = "chat"
)

// ModelConfig represents a model and its size.
type ModelConfig struct {
	Name string
	Size ModelSize
	ID   string // The actual model ID in LM Studio
}

// TaskRequirement defines what size model a task needs.
type TaskRequirement struct {
	Task          TaskType
	MinSize       ModelSize
	PreferredSize ModelSize
	Description   string
}

// Orchestrator manages multiple models working together.
type Orchestrator struct {
	modelsBySize map[ModelSize][]*ModelConfig
	clients      map[string]*llm.LMStudioClient
	tools        *tools.Registry
	taskReqs     map[TaskType]TaskRequirement
	taskList     *TaskList
	mainModel    string
}

// NewOrchestrator creates a new model orchestrator.
func NewOrchestrator(mainModel string, tools *tools.Registry) *Orchestrator {
	o := &Orchestrator{
		modelsBySize: make(map[ModelSize][]*ModelConfig),
		clients:      make(map[string]*llm.LMStudioClient),
		tools:        tools,
		mainModel:    mainModel,
		taskReqs:     make(map[TaskType]TaskRequirement),
		taskList:     NewTaskList(),
	}

	// Define task requirements
	o.setupTaskRequirements()

	return o
}

// setupTaskRequirements defines what size models different tasks need.
func (o *Orchestrator) setupTaskRequirements() {
	o.taskReqs = map[TaskType]TaskRequirement{
		TaskSummarize: {
			Task:          TaskSummarize,
			MinSize:       SizeXS,
			PreferredSize: SizeS,
			Description:   "Quick summaries and directory listings",
		},
		TaskAnalyze: {
			Task:          TaskAnalyze,
			MinSize:       SizeS,
			PreferredSize: SizeM,
			Description:   "Parse user messages into actionable plans",
		},
		TaskCode: {
			Task:          TaskCode,
			MinSize:       SizeM,
			PreferredSize: SizeL,
			Description:   "Complex coding tasks and problem solving",
		},
		TaskEdit: {
			Task:          TaskEdit,
			MinSize:       SizeS,
			PreferredSize: SizeM,
			Description:   "Simple file edits and updates",
		},
		TaskReview: {
			Task:          TaskReview,
			MinSize:       SizeM,
			PreferredSize: SizeL,
			Description:   "Compare multiple solutions and pick the best",
		},
		TaskChat: {
			Task:          TaskChat,
			MinSize:       SizeS,
			PreferredSize: SizeM,
			Description:   "General conversation and explanations",
		},
	}
}

// RegisterModel adds a model with its size classification.
func (o *Orchestrator) RegisterModel(config ModelConfig) {
	o.modelsBySize[config.Size] = append(o.modelsBySize[config.Size], &config)

	// Create client for this model
	client := llm.NewLMStudioClient()
	client.SetModel(config.ID)
	o.clients[config.ID] = client
}

// SetupDefaultModels configures common models by size.
func (o *Orchestrator) SetupDefaultModels() {
	// XS models (< 2B) - Super fast for summaries
	xsModels := []ModelConfig{
		{Name: "Llama 3.2 1B", Size: SizeXS, ID: "llama-3.2-1b-instruct"},
		{Name: "Qwen 2.5 0.5B", Size: SizeXS, ID: "qwen2.5-0.5b-instruct"},
	}

	// S models (2-4B) - Fast analysis
	sModels := []ModelConfig{
		{Name: "Phi-3 Mini", Size: SizeS, ID: "phi-3-mini-4k-instruct"},
		{Name: "Qwen 2.5 Coder 1.5B", Size: SizeS, ID: "qwen2.5-coder-1.5b"},
		{Name: "Gemma 2B", Size: SizeS, ID: "gemma-2b-it"},
	}

	// M models (7-13B) - Balanced
	mModels := []ModelConfig{
		{Name: "Mistral 7B", Size: SizeM, ID: "mistral-7b-instruct"},
		{Name: "DeepSeek Coder 7B", Size: SizeM, ID: "deepseek-coder-v2:7b"},
		{Name: "Qwen 2.5 7B", Size: SizeM, ID: "qwen2.5-7b-instruct"},
	}

	// L models (14-34B) - Powerful coding
	lModels := []ModelConfig{
		{Name: "DeepSeek Coder 16B", Size: SizeL, ID: "deepseek-coder-v2:16b"},
		{Name: "Qwen 2.5 Coder 14B", Size: SizeL, ID: "qwen2.5-coder-14b"},
		{Name: "Codestral 22B", Size: SizeL, ID: "codestral-22b"},
		{Name: "Mixtral 8x7B", Size: SizeL, ID: "mixtral-8x7b-instruct"},
	}

	// Register all models
	for _, m := range xsModels {
		o.RegisterModel(m)
	}
	for _, m := range sModels {
		o.RegisterModel(m)
	}
	for _, m := range mModels {
		o.RegisterModel(m)
	}
	for _, m := range lModels {
		o.RegisterModel(m)
	}
}

// GetModelForTask returns the best available model for a task type.
func (o *Orchestrator) GetModelForTask(task TaskType) (*llm.LMStudioClient, error) {
	req, exists := o.taskReqs[task]
	if !exists {
		return nil, fmt.Errorf("unknown task type: %s", task)
	}

	// Try preferred size first
	if models, ok := o.modelsBySize[req.PreferredSize]; ok && len(models) > 0 {
		return o.clients[models[0].ID], nil
	}

	// Fall back to minimum size
	if models, ok := o.modelsBySize[req.MinSize]; ok && len(models) > 0 {
		return o.clients[models[0].ID], nil
	}

	// Try any larger size
	sizes := []ModelSize{SizeS, SizeM, SizeL, SizeXL}
	for _, size := range sizes {
		if size >= req.MinSize {
			if models, ok := o.modelsBySize[size]; ok && len(models) > 0 {
				return o.clients[models[0].ID], nil
			}
		}
	}

	// Fallback to main model
	client := llm.NewLMStudioClient()
	client.SetModel(o.mainModel)
	return client, nil
}

// AnalyzeRequest analyzes a user request and decides which models to use.
func (o *Orchestrator) AnalyzeRequest(ctx context.Context, request string) (*WorkPlan, error) {
	// Use a small model for quick analysis
	analyst, err := o.GetModelForTask(TaskAnalyze)
	if err != nil {
		// Fallback to main model
		analyst = llm.NewLMStudioClient()
		analyst.SetModel(o.mainModel)
	}

	prompt := fmt.Sprintf(`Analyze this request and determine what tools and expertise are needed.

Request: %s

Available tools:
- read_file: Read contents of files
- write_file: Create or update files
- list_directory: List directory contents

Respond with a work plan that includes:
1. What needs to be done
2. Which tools to use
3. Whether this needs coding expertise (yes/no)
4. Whether this needs summarization (yes/no)

Be concise.`, request)

	messages := []llm.Message{
		{Role: "system", Content: "You are a task analyzer. Be concise and specific."},
		{Role: "user", Content: prompt},
	}

	response, err := analyst.Complete(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Parse the response to determine the plan
	plan := &WorkPlan{
		Request:       request,
		AnalysisNotes: response,
	}

	return plan, nil
}

// ExecuteWithBestModel executes a task with the most appropriate model.
func (o *Orchestrator) ExecuteWithBestModel(ctx context.Context, plan *WorkPlan, systemPrompt, userPrompt string) (string, error) {
	// For now, just use the main model
	// TODO: Implement model selection based on task analysis
	client := llm.NewLMStudioClient()
	client.SetModel(o.mainModel)

	// Add tool descriptions to system prompt
	toolInfo := o.getToolInfo()
	enhancedSystemPrompt := systemPrompt + "\n\n" + toolInfo

	messages := []llm.Message{
		{Role: "system", Content: enhancedSystemPrompt},
		{Role: "user", Content: userPrompt},
	}

	return client.Complete(ctx, messages)
}

func (o *Orchestrator) getToolInfo() string {
	var sb strings.Builder
	sb.WriteString("Available tools:\n")

	for _, desc := range o.tools.GetToolDescriptions() {
		sb.WriteString(fmt.Sprintf("\n%s:\n%s\n", desc["name"], desc["description"]))
	}

	return sb.String()
}

// WorkPlan represents an analysis of what needs to be done.
type WorkPlan struct {
	Request       string
	AnalysisNotes string
	Tasks         []*Task
}

// CreatePlan uses a medium model to analyze request and create a task plan.
func (o *Orchestrator) CreatePlan(ctx context.Context, request string) (*WorkPlan, error) {
	// Use a medium model to parse and plan
	planner, err := o.GetModelForTask(TaskAnalyze)
	if err != nil {
		return nil, err
	}

	// Create initial analysis task
	analysisTask := o.taskList.Add(TaskAnalyze, "Analyze user request", request)
	if err := o.taskList.UpdateStatus(analysisTask.ID, StatusInProgress); err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	prompt := fmt.Sprintf(`Analyze this request and create a step-by-step plan.

Request: %s

Available tools:
%s

For each step, specify:
1. What needs to be done
2. Which tool(s) to use
3. What size model is best (XS/S/M/L)

Be specific and actionable.`, request, o.getToolInfo())

	messages := []llm.Message{
		{Role: "system", Content: "You are a task planner. Break down requests into specific, actionable steps."},
		{Role: "user", Content: prompt},
	}

	response, err := planner.Complete(ctx, messages)
	if err != nil {
		o.taskList.SetError(analysisTask.ID, err)
		return nil, err
	}

	if err := o.taskList.SetOutput(analysisTask.ID, response); err != nil {
		return nil, fmt.Errorf("failed to set task output: %w", err)
	}
	if err := o.taskList.UpdateStatus(analysisTask.ID, StatusCompleted); err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	// For now, create a simple plan
	// In a real implementation, we'd parse the response to create specific tasks
	plan := &WorkPlan{
		Request:       request,
		Tasks:         []*Task{analysisTask},
		AnalysisNotes: response,
	}

	// Example: if the response mentions "read file", create a read task
	if strings.Contains(strings.ToLower(response), "read") {
		readTask := o.taskList.Add(TaskEdit, "Read requested files", request)
		plan.Tasks = append(plan.Tasks, readTask)
	}

	if strings.Contains(strings.ToLower(response), "write") || strings.Contains(strings.ToLower(response), "create") {
		writeTask := o.taskList.Add(TaskCode, "Write/create files", request)
		plan.Tasks = append(plan.Tasks, writeTask)
	}

	return plan, nil
}

// ExecutePlan runs through all tasks in a plan.
func (o *Orchestrator) ExecutePlan(ctx context.Context, plan *WorkPlan) error {
	for {
		// Get next task
		task := o.taskList.GetNext()
		if task == nil {
			break // No more tasks
		}

		// Get appropriate model for task
		model, err := o.GetModelForTask(task.Type)
		if err != nil {
			o.taskList.SetError(task.ID, err)
			continue
		}

		// Update status
		if err := o.taskList.UpdateStatus(task.ID, StatusInProgress); err != nil {
			fmt.Printf("Failed to update task status: %v\n", err)
		}

		// Execute task
		// In real implementation, this would use the tools
		messages := []llm.Message{
			{Role: "system", Content: "Execute the task using available tools.\n\n" + o.getToolInfo()},
			{Role: "user", Content: task.Input},
		}

		response, err := model.Complete(ctx, messages)
		if err != nil {
			o.taskList.SetError(task.ID, err)
			continue
		}

		if err := o.taskList.SetOutput(task.ID, response); err != nil {
			fmt.Printf("Failed to set task output: %v\n", err)
		}
		if err := o.taskList.UpdateStatus(task.ID, StatusCompleted); err != nil {
			fmt.Printf("Failed to update task status: %v\n", err)
		}
	}

	return nil
}

// GetTaskSummary returns the current state of all tasks.
func (o *Orchestrator) GetTaskSummary() string {
	return o.taskList.Summary()
}
