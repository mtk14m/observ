package auth

import "context"

// PasswordHashForTest exposes the stored hash to tests.
func (s *Store) PasswordHashForTest(ctx context.Context, email string) (string, error) {
	var h string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE email = ?`, email).Scan(&h)
	return h, err
}
