// Пакет bd — адаптер к CLI bd (beads): чтение с --readonly, разбор JSON и записи:
// comments add, init, config set и config unset.
package bd

import "time"

// Dep — связь задачи с другой задачей.
type Dep struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

// Issue — задача из bd list.
type Issue struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Notes        string     `json:"notes"`
	Status       string     `json:"status"`
	Priority     int        `json:"priority"`
	IssueType    string     `json:"issue_type"`
	Assignee     string     `json:"assignee"`
	Owner        string     `json:"owner"`
	Labels       []string   `json:"labels"`
	Parent       string     `json:"parent"`
	Dependencies []Dep      `json:"dependencies"`
	CommentCount int        `json:"comment_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ClosedAt     *time.Time `json:"closed_at"`
	CloseReason  string     `json:"close_reason"`
	// Поля спеки (v1.4): в bd 0.59 omitempty, приходят и в list, и в show.
	SpecID             string `json:"spec_id"`
	Design             string `json:"design"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	// IsTemplate — шаблон-молекула bd: такие задачи bd list отдаёт только с --include-templates.
	IsTemplate bool `json:"is_template"`
}

// Related — связанная задача в bd show.
type Related struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	Priority       int    `json:"priority"`
	IssueType      string `json:"issue_type"`
	DependencyType string `json:"dependency_type"`
}

// Detail — связи задачи из bd show и поля спеки (bd show --json отдаёт задачу целиком;
// карточка берёт их отсюда, а до ответа — из списка).
type Detail struct {
	Dependencies       []Related `json:"dependencies"`
	Dependents         []Related `json:"dependents"`
	SpecID             string    `json:"spec_id"`
	Design             string    `json:"design"`
	AcceptanceCriteria string    `json:"acceptance_criteria"`
}

// Comment — комментарий к задаче.
type Comment struct {
	ID        int       `json:"id"`
	IssueID   string    `json:"issue_id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// TypeInfo — тип задачи из bd types.
type TypeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Memory — запись постоянной памяти bd (bd memories): ключ и текст (markdown).
type Memory struct {
	Key  string
	Text string
}

// Formula — формула из bd formula list --json: имя, тип, описание и путь к файлу.
type Formula struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Source      string `json:"source"`
}
