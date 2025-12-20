package supervisor

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/CLIAIMONITOR/internal/memory"
	"gopkg.in/yaml.v3"
)

// Scanner discovers and parses workflow files in a repository
type Scanner struct {
	memDB memory.MemoryDB
}

// NewScanner creates a new workflow scanner
func NewScanner(memDB memory.MemoryDB) *Scanner {
	return &Scanner{
		memDB: memDB,
	}
}

// ScanResult contains the results of scanning a repository
type ScanResult struct {
	RepoID          string
	CLAUDEmd        *CLAUDEmdContext
	WorkflowFiles   []*WorkflowFile
	PlanFiles       []*PlanFile
	DiscoveredTasks []*memory.WorkflowTask
}

// CLAUDEmdContext represents parsed CLAUDE.md content
type CLAUDEmdContext struct {
	FilePath    string
	Content     string
	Projects    []string
	Technologies map[string]string
	KeySections  map[string]string
}

// WorkflowFile represents a GitHub workflow YAML file
type WorkflowFile struct {
	FilePath string
	Content  string
	Name     string
}

// PlanFile represents a plan YAML file (e.g., docs/plans/*.yaml)
type PlanFile struct {
	FilePath string
	Content  string
	Tasks    []PlanTask
}

// PlanTask represents a task from a plan file
type PlanTask struct {
	ID          string
	Title       string
	Description string
	Priority    string
	Status      string
	Tags        []string
}

// ScanForWorkflows scans a repository for workflow-related files
func (s *Scanner) ScanForWorkflows(repoID string) (*ScanResult, error) {
	// Get repository info
	repo, err := s.memDB.GetRepo(repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}

	result := &ScanResult{
		RepoID:        repoID,
		WorkflowFiles: []*WorkflowFile{},
		PlanFiles:     []*PlanFile{},
		DiscoveredTasks: []*memory.WorkflowTask{},
	}

	basePath := repo.BasePath

	// 1. Look for CLAUDE.md
	claudePath := filepath.Join(basePath, "CLAUDE.md")
	if fileExists(claudePath) {
		claudeCtx, err := s.ParseCLAUDEmd(claudePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CLAUDE.md: %w", err)
		}
		result.CLAUDEmd = claudeCtx

		// Store in memory.db
		if err := s.storeRepoFile(repoID, "CLAUDE.md", "claude_md", claudeCtx.Content); err != nil {
			return nil, fmt.Errorf("failed to store CLAUDE.md: %w", err)
		}
	}

	// 2. Look for docs/plans/*.yaml
	plansDir := filepath.Join(basePath, "docs", "plans")
	if dirExists(plansDir) {
		planFiles, err := s.scanPlanFiles(plansDir)
		if err != nil {
			return nil, fmt.Errorf("failed to scan plan files: %w", err)
		}
		result.PlanFiles = planFiles

		// Parse tasks from plan files
		for _, planFile := range planFiles {
			tasks, err := s.ParseWorkflowYAML(repoID, planFile.FilePath, planFile.Content)
			if err != nil {
				log.Printf("[SCANNER] Skipping plan file %s: not a valid task list (%v)", planFile.FilePath, err)
				continue
			}
			result.DiscoveredTasks = append(result.DiscoveredTasks, tasks...)

			// Store in memory.db
			relativePath := strings.TrimPrefix(planFile.FilePath, basePath)
			relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))
			if err := s.storeRepoFile(repoID, relativePath, "plan_yaml", planFile.Content); err != nil {
				return nil, fmt.Errorf("failed to store plan file: %w", err)
			}
		}
	}

	// 3. Look for .github/workflows/*.yaml
	workflowsDir := filepath.Join(basePath, ".github", "workflows")
	if dirExists(workflowsDir) {
		workflowFiles, err := s.scanWorkflowFiles(workflowsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow files: %w", err)
		}
		result.WorkflowFiles = workflowFiles

		// Store in memory.db
		for _, wf := range workflowFiles {
			relativePath := strings.TrimPrefix(wf.FilePath, basePath)
			relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))
			if err := s.storeRepoFile(repoID, relativePath, "workflow_yaml", wf.Content); err != nil {
				return nil, fmt.Errorf("failed to store workflow file: %w", err)
			}
		}
	}

	// Store all discovered tasks in memory.db
	if len(result.DiscoveredTasks) > 0 {
		if err := s.memDB.CreateTasks(result.DiscoveredTasks); err != nil {
			return nil, fmt.Errorf("failed to store tasks: %w", err)
		}
	}

	// Mark repo as scanned
	if err := s.memDB.UpdateRepoScan(repoID); err != nil {
		return nil, fmt.Errorf("failed to update repo scan time: %w", err)
	}

	return result, nil
}

// ParseCLAUDEmd extracts context from CLAUDE.md
func (s *Scanner) ParseCLAUDEmd(path string) (*CLAUDEmdContext, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read CLAUDE.md: %w", err)
	}

	ctx := &CLAUDEmdContext{
		FilePath:     path,
		Content:      string(content),
		Technologies: make(map[string]string),
		KeySections:  make(map[string]string),
	}

	// Simple parsing - extract key information
	lines := strings.Split(string(content), "\n")
	var currentSection string
	var sectionContent strings.Builder

	for _, line := range lines {
		// Detect section headers (## or ###)
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if currentSection != "" {
				ctx.KeySections[currentSection] = strings.TrimSpace(sectionContent.String())
			}
			currentSection = strings.TrimPrefix(line, "## ")
			sectionContent.Reset()
		} else if currentSection != "" {
			sectionContent.WriteString(line)
			sectionContent.WriteString("\n")
		}

		// Extract project names (look for mentions of project names in caps)
		if strings.Contains(line, "Project") || strings.Contains(line, "project") {
			// Basic extraction - can be enhanced with better parsing
		}
	}

	// Save last section
	if currentSection != "" {
		ctx.KeySections[currentSection] = strings.TrimSpace(sectionContent.String())
	}

	return ctx, nil
}

// ParseWorkflowYAML extracts workflow tasks from YAML files
func (s *Scanner) ParseWorkflowYAML(repoID, filePath, content string) ([]*memory.WorkflowTask, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var tasks []*memory.WorkflowTask

	// Try to find tasks in the YAML structure
	// Support various formats:
	// 1. tasks: [...]
	// 2. workflow: { tasks: [...] }
	// 3. plan: { tasks: [...] }

	tasksList := extractTasksList(data)
	if tasksList == nil {
		return nil, fmt.Errorf("no tasks found in YAML")
	}

	for _, taskData := range tasksList {
		task := parseTaskFromMap(repoID, filePath, taskData)
		if task != nil {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// Helper functions

func (s *Scanner) storeRepoFile(repoID, filePath, fileType, content string) error {
	hash := hashContent(content)
	file := &memory.RepoFile{
		RepoID:      repoID,
		FilePath:    filePath,
		FileType:    fileType,
		ContentHash: hash,
		Content:     content,
	}
	return s.memDB.StoreRepoFile(file)
}

func (s *Scanner) scanPlanFiles(dir string) ([]*PlanFile, error) {
	var files []*PlanFile

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .yaml and .yml files
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[SCANNER] Failed to read file %s: %v", path, err)
			continue
		}

		files = append(files, &PlanFile{
			FilePath: path,
			Content:  string(content),
		})
	}

	return files, nil
}

func (s *Scanner) scanWorkflowFiles(dir string) ([]*WorkflowFile, error) {
	var files []*WorkflowFile

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .yaml and .yml files
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[SCANNER] Failed to read file %s: %v", path, err)
			continue
		}

		files = append(files, &WorkflowFile{
			FilePath: path,
			Content:  string(content),
			Name:     strings.TrimSuffix(name, filepath.Ext(name)),
		})
	}

	return files, nil
}

func extractTasksList(data map[string]interface{}) []map[string]interface{} {
	// Try direct tasks key
	if tasks, ok := data["tasks"].([]interface{}); ok {
		return convertToMapSlice(tasks)
	}

	// Try workflow.tasks
	if workflow, ok := data["workflow"].(map[string]interface{}); ok {
		if tasks, ok := workflow["tasks"].([]interface{}); ok {
			return convertToMapSlice(tasks)
		}
	}

	// Try plan.tasks
	if plan, ok := data["plan"].(map[string]interface{}); ok {
		if tasks, ok := plan["tasks"].([]interface{}); ok {
			return convertToMapSlice(tasks)
		}
	}

	return nil
}

func convertToMapSlice(data []interface{}) []map[string]interface{} {
	var result []map[string]interface{}
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

func parseTaskFromMap(repoID string, sourceFile string, data map[string]interface{}) *memory.WorkflowTask {
	// Extract ID (required)
	id, _ := data["id"].(string)
	if id == "" {
		return nil
	}

	// Extract other fields
	title, _ := data["title"].(string)
	description, _ := data["description"].(string)
	priority, _ := data["priority"].(string)
	status, _ := data["status"].(string)

	if priority == "" {
		priority = "medium"
	}
	if status == "" {
		status = "pending"
	}

	task := &memory.WorkflowTask{
		ID:          id,
		RepoID:      repoID,
		SourceFile:  sourceFile,
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      status,
	}

	// Optional fields
	if assignedAgent, ok := data["assigned_agent_id"].(string); ok {
		task.AssignedAgentID = assignedAgent
	}
	if parentTask, ok := data["parent_task_id"].(string); ok {
		task.ParentTaskID = parentTask
	}
	if effort, ok := data["estimated_effort"].(string); ok {
		task.EstimatedEffort = effort
	}

	return task
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func hashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
