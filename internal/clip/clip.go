// Пакет clip копирует текст в буфер обмена: через pbcopy, а без него — через OSC52.
package clip

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/muesli/termenv"
)

// pbcopyTimeout ограничивает pbcopy: Copy вызывается из Update и не должна вешать интерфейс.
const pbcopyTimeout = 3 * time.Second

// Copy кладёт s в буфер обмена. Если pbcopy есть в PATH, вызывает его со stdin = s
// (не дольше pbcopyTimeout) и возвращает его ошибку. Иначе пишет OSC52 в терминал
// (работает и по SSH) и возвращает nil: доставку OSC52 проверить нельзя.
func Copy(s string) error {
	path, err := exec.LookPath("pbcopy")
	if err != nil {
		termenv.DefaultOutput().Copy(s)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), pbcopyTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}
