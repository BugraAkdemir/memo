package twin

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ReplyModel is deliberately independent of decision settings and ordinary
// preferences. Changing channel switches cannot overwrite model selection.
type ReplyModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func (s *Store) ReplyModel(ctx context.Context) (ReplyModel, error) {
	var m ReplyModel
	err := s.db.QueryRowContext(ctx, "SELECT provider,model FROM twin_reply_model WHERE id=1").Scan(&m.Provider, &m.Model)
	if errors.Is(err, sql.ErrNoRows) {
		return m, nil
	}
	return m, err
}
func (s *Store) SaveReplyModel(ctx context.Context, m ReplyModel) error {
	m.Provider = strings.TrimSpace(m.Provider)
	m.Model = strings.TrimSpace(m.Model)
	if (m.Provider == "") != (m.Model == "") || len(m.Provider) > 300 || len(m.Model) > 512 || strings.ContainsAny(m.Provider+m.Model, "\r\n\x00") {
		return ErrInvalid
	}
	return s.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO twin_reply_model(id,provider,model) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET provider=excluded.provider,model=excluded.model", m.Provider, m.Model)
		if err != nil {
			return err
		}
		return audit(ctx, tx, "reply_model_updated", "")
	})
}
