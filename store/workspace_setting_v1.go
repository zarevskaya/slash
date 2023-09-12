package store

import (
	"context"
	"errors"
	"strconv"
	"strings"

	storepb "github.com/boojack/slash/proto/gen/store"
	"google.golang.org/protobuf/encoding/protojson"
)

type FindWorkspaceSettingV1 struct {
	Key storepb.WorkspaceSettingKey
}

func (s *Store) UpsertWorkspaceSettingV1(ctx context.Context, upsert *storepb.WorkspaceSetting) (*storepb.WorkspaceSetting, error) {
	stmt := `
		INSERT INTO workspace_setting (
			key,
			value
		)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE 
		SET value = EXCLUDED.value
	`
	var valueString string
	if upsert.Key == storepb.WorkspaceSettingKey_WORKSPACE_SETTING_SECRET_SESSION {
		valueString = upsert.GetSecretSession()
	} else if upsert.Key == storepb.WorkspaceSettingKey_WORKSAPCE_SETTING_ENABLE_SIGNUP {
		valueString = strconv.FormatBool(upsert.GetEnableSignup())
	} else if upsert.Key == storepb.WorkspaceSettingKey_WORKSPACE_SETTING_RESOURCE_RELATIVE_PATH {
		valueString = upsert.GetResourceRelativePath()
	} else if upsert.Key == storepb.WorkspaceSettingKey_WORKSPACE_SETTING_AUTO_BACKUP {
		valueBytes, err := protojson.Marshal(upsert.GetAutoBackup())
		if err != nil {
			return nil, err
		}
		valueString = string(valueBytes)
	} else {
		return nil, errors.New("invalid workspace setting key")
	}

	if _, err := s.db.ExecContext(ctx, stmt, upsert.Key.String(), valueString); err != nil {
		return nil, err
	}

	workspaceSetting := upsert
	s.workspaceSettingCache.Store(workspaceSetting.Key, workspaceSetting)
	return workspaceSetting, nil
}

func (s *Store) ListWorkspaceSettingsV1(ctx context.Context, find *FindWorkspaceSettingV1) ([]*storepb.WorkspaceSetting, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.Key != storepb.WorkspaceSettingKey_WORKSPACE_SETTING_KEY_UNSPECIFIED {
		where, args = append(where, "key = ?"), append(args, find.Key.String())
	}

	query := `
		SELECT
			key,
			value
		FROM workspace_setting
		WHERE ` + strings.Join(where, " AND ")
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	list := []*storepb.WorkspaceSetting{}
	for rows.Next() {
		workspaceSetting := &storepb.WorkspaceSetting{}
		var keyString, valueString string
		if err := rows.Scan(
			&keyString,
			&valueString,
		); err != nil {
			return nil, err
		}
		workspaceSetting.Key = storepb.WorkspaceSettingKey(storepb.WorkspaceSettingKey_value[keyString])
		if workspaceSetting.Key == storepb.WorkspaceSettingKey_WORKSPACE_SETTING_SECRET_SESSION {
			workspaceSetting.Value = &storepb.WorkspaceSetting_SecretSession{SecretSession: valueString}
		} else if workspaceSetting.Key == storepb.WorkspaceSettingKey_WORKSAPCE_SETTING_ENABLE_SIGNUP {
			enableSignup, err := strconv.ParseBool(valueString)
			if err != nil {
				return nil, err
			}
			workspaceSetting.Value = &storepb.WorkspaceSetting_EnableSignup{EnableSignup: enableSignup}
		} else if workspaceSetting.Key == storepb.WorkspaceSettingKey_WORKSPACE_SETTING_RESOURCE_RELATIVE_PATH {
			workspaceSetting.Value = &storepb.WorkspaceSetting_ResourceRelativePath{ResourceRelativePath: valueString}
		} else if workspaceSetting.Key == storepb.WorkspaceSettingKey_WORKSPACE_SETTING_AUTO_BACKUP {
			autoBackupSetting := &storepb.AutoBackupWorkspaceSetting{}
			if err := protojson.Unmarshal([]byte(valueString), autoBackupSetting); err != nil {
				return nil, err
			}
			workspaceSetting.Value = &storepb.WorkspaceSetting_AutoBackup{AutoBackup: autoBackupSetting}
		} else {
			return nil, errors.New("invalid workspace setting key")
		}

		list = append(list, workspaceSetting)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, workspaceSetting := range list {
		s.workspaceSettingCache.Store(workspaceSetting.Key, workspaceSetting)
	}

	return list, nil
}

func (s *Store) GetWorkspaceSettingV1(ctx context.Context, find *FindWorkspaceSettingV1) (*storepb.WorkspaceSetting, error) {
	if find.Key != storepb.WorkspaceSettingKey_WORKSAPCE_SETTING_ENABLE_SIGNUP {
		if cache, ok := s.workspaceSettingCache.Load(find.Key); ok {
			return cache.(*storepb.WorkspaceSetting), nil
		}
	}

	list, err := s.ListWorkspaceSettingsV1(ctx, find)
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, nil
	}

	workspaceSetting := list[0]
	s.workspaceSettingCache.Store(workspaceSetting.Key, workspaceSetting)
	return workspaceSetting, nil
}