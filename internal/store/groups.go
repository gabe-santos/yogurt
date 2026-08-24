package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoGroup reports a Group that is not there.
var ErrNoGroup = errors.New("no such Group")

// ErrDefaultGroup reports an attempt to delete the default Group, which must
// always exist so that no Feed is ever unreachable.
var ErrDefaultGroup = errors.New("the default Group cannot be deleted")

// Group is a named set of Feeds, used to scope reading to one part of the
// collection. A Feed belongs to exactly one Group.
type Group struct {
	ID        int64
	Name      string
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateGroup stores a new Group and returns it with its assigned id.
func (s *Store) CreateGroup(ctx context.Context, name string, now time.Time) (Group, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO groups (name, is_default, created_at, updated_at) VALUES (?, 0, ?, ?) RETURNING id`,
		name, now.Unix(), now.Unix()).Scan(&id)
	if err != nil {
		return Group{}, fmt.Errorf("create group %q: %w", name, err)
	}
	return Group{ID: id, Name: name, CreatedAt: now.UTC().Truncate(time.Second), UpdatedAt: now.UTC().Truncate(time.Second)}, nil
}

// Groups reads the whole collection, by name, default Group first.
func (s *Store) Groups(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, is_default, created_at, updated_at
		 FROM groups ORDER BY is_default DESC, name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("read groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("read groups: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func scanGroup(row rowScanner) (Group, error) {
	var group Group
	var isDefault int64
	var createdAt, updatedAt int64
	if err := row.Scan(&group.ID, &group.Name, &isDefault, &createdAt, &updatedAt); err != nil {
		return Group{}, err
	}
	group.IsDefault = isDefault != 0
	group.CreatedAt = time.Unix(createdAt, 0).UTC()
	group.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return group, nil
}

// defaultGroupID is the id of the Group every unsorted Feed belongs to.
func (s *Store) defaultGroupID(ctx context.Context) (int64, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM groups WHERE is_default = 1`).Scan(&id); err != nil {
		return 0, fmt.Errorf("read default group: %w", err)
	}
	return id, nil
}

// RenameGroup sets a Group's name and returns it as stored. It returns
// ErrNoGroup when there is no such Group.
func (s *Store) RenameGroup(ctx context.Context, id int64, name string, now time.Time) (Group, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE groups SET name = ?, updated_at = ? WHERE id = ?`, name, now.Unix(), id)
	if err != nil {
		return Group{}, fmt.Errorf("rename group %d: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Group{}, fmt.Errorf("rename group %d: %w", id, err)
	}
	if affected == 0 {
		return Group{}, ErrNoGroup
	}

	return s.Group(ctx, id)
}

// Group reads one Group by id. It returns ErrNoGroup when there is no such
// Group.
func (s *Store) Group(ctx context.Context, id int64) (Group, error) {
	group, err := scanGroup(s.db.QueryRowContext(ctx,
		`SELECT id, name, is_default, created_at, updated_at FROM groups WHERE id = ?`, id))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Group{}, ErrNoGroup
	case err != nil:
		return Group{}, fmt.Errorf("read group %d: %w", id, err)
	}
	return group, nil
}

// DeleteGroup removes a Group, reparenting its Feeds to the default Group
// rather than deleting them, in one transaction. It returns ErrNoGroup when
// there is no such Group, and ErrDefaultGroup when asked to delete the default
// Group.
func (s *Store) DeleteGroup(ctx context.Context, id int64, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete group %d: %w", id, err)
	}
	defer tx.Rollback()

	var isDefault int64
	err = tx.QueryRowContext(ctx, `SELECT is_default FROM groups WHERE id = ?`, id).Scan(&isDefault)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNoGroup
	case err != nil:
		return fmt.Errorf("delete group %d: %w", id, err)
	}
	if isDefault != 0 {
		return ErrDefaultGroup
	}

	var defaultID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM groups WHERE is_default = 1`).Scan(&defaultID); err != nil {
		return fmt.Errorf("delete group %d: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE feeds SET group_id = ?, updated_at = ? WHERE group_id = ?`,
		defaultID, now.Unix(), id); err != nil {
		return fmt.Errorf("delete group %d: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete group %d: %w", id, err)
	}
	return tx.Commit()
}

// GroupUnreadCounts reads the number of unread Entries per Group, summed
// across the Feeds each one holds, omitting a Group with none.
func (s *Store) GroupUnreadCounts(ctx context.Context) (map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT f.group_id, COUNT(*)
		 FROM entries e JOIN feeds f ON f.id = e.feed_id
		 WHERE e.read = 0
		 GROUP BY f.group_id`)
	if err != nil {
		return nil, fmt.Errorf("read group unread counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var groupID int64
		var count int
		if err := rows.Scan(&groupID, &count); err != nil {
			return nil, fmt.Errorf("read group unread counts: %w", err)
		}
		counts[groupID] = count
	}
	return counts, rows.Err()
}
