package portal

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/model"
)

func (s *Service) backfillUsageDepartmentSnapshots(ctx context.Context) error {
	records, err := s.db.GetAll(ctx, `
		SELECT m.consumer_name
		FROM org_account_membership m
		LEFT JOIN org_department d ON d.department_id = m.department_id`)
	if err != nil {
		return gerror.Wrap(err, "query organization memberships failed")
	}
	if len(records) == 0 {
		return nil
	}
	return s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, record := range records {
			consumerName := model.NormalizeUsername(record["consumer_name"].String())
			if consumerName == "" {
				continue
			}
			orgContext, ctxErr := s.loadUserOrgContext(ctx, consumerName)
			if ctxErr != nil {
				return ctxErr
			}
			if _, txErr := tx.Exec(`
				UPDATE billing_usage_event
				SET department_id = ?, department_path = ?
				WHERE consumer_name = ?
				  AND department_id = ''`,
				orgContext.DepartmentID,
				orgContext.DepartmentPath,
				consumerName,
			); txErr != nil {
				return gerror.Wrap(txErr, "backfill billing usage event department failed")
			}
			if _, txErr := tx.Exec(`
				UPDATE portal_usage_daily
				SET department_id = ?, department_path = ?
				WHERE consumer_name = ?
				  AND department_id = ''`,
				orgContext.DepartmentID,
				orgContext.DepartmentPath,
				consumerName,
			); txErr != nil {
				return gerror.Wrap(txErr, "backfill portal usage daily department failed")
			}
		}
		return nil
	})
}

func buildStringInQuery(queryTemplate string, values []string) (string, []any) {
	placeholders := make([]string, 0, len(values))
	args := make([]any, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, normalized)
	}
	if len(placeholders) == 0 {
		placeholders = append(placeholders, "''")
	}
	return fmt.Sprintf(queryTemplate, strings.Join(placeholders, ",")), args
}
