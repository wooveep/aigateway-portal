package portal

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/schema/shared"
)

const orgRootDepartmentID = "root"

type userOrgContext struct {
	DepartmentID      string
	DepartmentName    string
	DepartmentPath    string
	AdminConsumerName string
	IsDepartmentAdmin bool
}

func (s *Service) ensureOrganizationSchema(ctx context.Context) error {
	if err := shared.ApplyToGDBWithDriver(ctx, s.db, s.cfg.DBDriver); err != nil {
		return gerror.Wrap(err, "organization migration failed")
	}
	if err := s.ensureRootDepartment(ctx); err != nil {
		return err
	}
	if err := s.ensureMembershipRows(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensureRootDepartment(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO org_department
		(department_id, name, parent_department_id, admin_consumer_name, path, level, sort_order, status)
		VALUES (?, ?, NULL, NULL, ?, 0, 0, 'active')
		`+s.upsertClause([]string{"department_id"},
		s.assignExcluded("name"),
		s.assignExcluded("path"),
		s.assignExcluded("level"),
		s.assignExcluded("sort_order"),
		s.assignExcluded("status"))+``,
		orgRootDepartmentID,
		"ROOT",
		orgRootDepartmentID,
	)
	if err != nil {
		return gerror.Wrap(err, "ensure root department failed")
	}
	return nil
}

func (s *Service) ensureMembershipRows(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, `
		INSERT INTO org_account_membership (consumer_name, department_id)
		SELECT consumer_name, NULL
		FROM portal_user
		WHERE COALESCE(is_deleted, FALSE) = FALSE
		`+s.upsertClause([]string{"consumer_name"}, s.assignExcluded("consumer_name"))+``); err != nil {
		return gerror.Wrap(err, "ensure account membership rows failed")
	}
	return nil
}

func (s *Service) ensureMembershipForConsumer(ctx context.Context, consumerName string) error {
	normalizedConsumer := strings.TrimSpace(consumerName)
	if normalizedConsumer == "" {
		return nil
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO org_account_membership (consumer_name, department_id)
		VALUES (?, NULL)
		`+s.upsertClause([]string{"consumer_name"}, s.assignExcluded("consumer_name"))+``, normalizedConsumer); err != nil {
		return gerror.Wrap(err, "ensure consumer membership failed")
	}
	return nil
}

func (s *Service) loadUserOrgContext(ctx context.Context, consumerName string) (userOrgContext, error) {
	normalizedConsumer := strings.TrimSpace(consumerName)
	if normalizedConsumer == "" {
		return userOrgContext{}, nil
	}

	record, err := s.db.GetOne(ctx, `
		SELECT
			m.department_id,
			COALESCE(d.admin_consumer_name, '') AS admin_consumer_name
		FROM org_account_membership m
		LEFT JOIN org_department d ON d.department_id = m.department_id
		WHERE m.consumer_name = ?
		LIMIT 1`, normalizedConsumer)
	if err != nil {
		return userOrgContext{}, gerror.Wrap(err, "query account membership failed")
	}
	if record.IsEmpty() {
		return userOrgContext{}, nil
	}

	departmentID := strings.TrimSpace(record["department_id"].String())
	adminConsumerName := strings.TrimSpace(record["admin_consumer_name"].String())
	if departmentID == "" {
		return userOrgContext{
			AdminConsumerName: adminConsumerName,
		}, nil
	}

	departmentName, departmentPath, err := s.resolveDepartmentPath(ctx, departmentID)
	if err != nil {
		return userOrgContext{}, err
	}
	return userOrgContext{
		DepartmentID:      departmentID,
		DepartmentName:    departmentName,
		DepartmentPath:    departmentPath,
		AdminConsumerName: adminConsumerName,
		IsDepartmentAdmin: adminConsumerName != "" && adminConsumerName == normalizedConsumer,
	}, nil
}

func (s *Service) resolveDepartmentPath(ctx context.Context, departmentID string) (string, string, error) {
	currentDepartmentID := strings.TrimSpace(departmentID)
	if currentDepartmentID == "" {
		return "", "", nil
	}

	names := make([]string, 0, 4)
	guard := make(map[string]struct{}, 4)
	for currentDepartmentID != "" && currentDepartmentID != orgRootDepartmentID {
		if _, ok := guard[currentDepartmentID]; ok {
			return "", "", gerror.New("department cycle detected")
		}
		guard[currentDepartmentID] = struct{}{}

		record, err := s.db.GetOne(ctx, `
			SELECT name, parent_department_id
			FROM org_department
			WHERE department_id = ?
			LIMIT 1`, currentDepartmentID)
		if err != nil {
			return "", "", gerror.Wrap(err, "query department failed")
		}
		if record.IsEmpty() {
			break
		}
		names = append(names, strings.TrimSpace(record["name"].String()))
		currentDepartmentID = strings.TrimSpace(record["parent_department_id"].String())
	}
	if len(names) == 0 {
		return "", "", nil
	}
	for left, right := 0, len(names)-1; left < right; left, right = left+1, right-1 {
		names[left], names[right] = names[right], names[left]
	}
	return names[len(names)-1], strings.Join(names, " / "), nil
}

func (s *Service) listDepartmentIdsInSubtree(ctx context.Context, departmentID string) ([]string, error) {
	rootID := strings.TrimSpace(departmentID)
	if rootID == "" {
		return []string{}, nil
	}
	rootPath, err := s.getDepartmentTreePath(ctx, rootID)
	if err != nil {
		return nil, err
	}
	if rootPath == "" {
		return []string{}, nil
	}

	records, err := s.db.GetAll(ctx, `
		SELECT department_id, path
		FROM org_department
		WHERE department_id = ?
		   OR path = ?
		   OR path LIKE ?`,
		rootID,
		rootPath,
		rootPath+"/%",
	)
	if err != nil {
		return nil, gerror.Wrap(err, "query department subtree failed")
	}
	items := make([]string, 0, len(records))
	for _, record := range records {
		deptID := strings.TrimSpace(record["department_id"].String())
		if deptID != "" {
			items = append(items, deptID)
		}
	}
	return items, nil
}

func (s *Service) listDepartmentAncestorIds(ctx context.Context, departmentID string) ([]string, error) {
	current := strings.TrimSpace(departmentID)
	if current == "" {
		return []string{}, nil
	}
	guard := map[string]struct{}{}
	items := make([]string, 0, 4)
	for current != "" && current != orgRootDepartmentID {
		if _, ok := guard[current]; ok {
			return nil, gerror.New("department cycle detected")
		}
		guard[current] = struct{}{}
		items = append(items, current)
		record, err := s.db.GetOne(ctx, `
			SELECT parent_department_id
			FROM org_department
			WHERE department_id = ?
			LIMIT 1`, current)
		if err != nil {
			return nil, gerror.Wrap(err, "query department ancestor failed")
		}
		if record.IsEmpty() {
			break
		}
		current = strings.TrimSpace(record["parent_department_id"].String())
	}
	return items, nil
}

func (s *Service) getDepartmentTreePath(ctx context.Context, departmentID string) (string, error) {
	deptID := strings.TrimSpace(departmentID)
	if deptID == "" {
		return "", nil
	}
	names := make([]string, 0, 4)
	guard := map[string]struct{}{}
	current := deptID
	for current != "" && current != orgRootDepartmentID {
		if _, ok := guard[current]; ok {
			return "", gerror.New("department cycle detected")
		}
		guard[current] = struct{}{}
		record, err := s.db.GetOne(ctx, `
			SELECT name, parent_department_id
			FROM org_department
			WHERE department_id = ?
			LIMIT 1`, current)
		if err != nil {
			return "", gerror.Wrap(err, "query department path failed")
		}
		if record.IsEmpty() {
			break
		}
		names = append(names, strings.TrimSpace(record["name"].String()))
		current = strings.TrimSpace(record["parent_department_id"].String())
	}
	if len(names) == 0 {
		return "", nil
	}
	for left, right := 0, len(names)-1; left < right; left, right = left+1, right-1 {
		names[left], names[right] = names[right], names[left]
	}
	return fmt.Sprintf("%s", strings.Join(names, "/")), nil
}
