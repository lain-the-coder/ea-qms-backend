-- +goose Up
-- +goose StatementBegin
INSERT INTO users (full_name, email, hashed_password, role)
VALUES
  (
    'System Administrator',
    'admin@eaqms.local',
    '$argon2id$v=19$m=65536,t=3,p=2$oneH5B/0uc+jZD7nnxDq0Q$KkmpBOeGIH6aC2IJqgwBTetordtTcBQ3U73ebXS8SFs',
    'Admin'
  ),
  (
    'Default CC Owner',
    'owner@eaqms.local',
    '$argon2id$v=19$m=65536,t=3,p=2$oneH5B/0uc+jZD7nnxDq0Q$KkmpBOeGIH6aC2IJqgwBTetordtTcBQ3U73ebXS8SFs',
    'CC Owner'
  ),
  (
    'Default Approver',
    'approver@eaqms.local',
    '$argon2id$v=19$m=65536,t=3,p=2$oneH5B/0uc+jZD7nnxDq0Q$KkmpBOeGIH6aC2IJqgwBTetordtTcBQ3U73ebXS8SFs',
    'Approver'
  ),
  (
    'Default Viewer',
    'viewer@eaqms.local',
    '$argon2id$v=19$m=65536,t=3,p=2$oneH5B/0uc+jZD7nnxDq0Q$KkmpBOeGIH6aC2IJqgwBTetordtTcBQ3U73ebXS8SFs',
    'Viewer'
  )
ON CONFLICT ((LOWER(email))) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM users WHERE LOWER(email) IN (
  'admin@eaqms.local',
  'owner@eaqms.local',
  'approver@eaqms.local',
  'viewer@eaqms.local'
);
-- +goose StatementEnd
