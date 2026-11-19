CREATE TABLE IF NOT EXISTS `teams` (
    `id` VARCHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `users` (
    `id` VARCHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    -- ユーザーを確かめた認証手段の ID と、その認証手段の中でユーザーを指す値
    `provider` VARCHAR(32) NOT NULL,
    `subject` VARCHAR(255) NOT NULL,
    -- NULL なら既定のアイコンを使う
    `icon_url` VARCHAR(2048),
    `team_id` VARCHAR(36),
    `is_admin` TINYINT(1) NOT NULL DEFAULT 0,
    PRIMARY KEY (`id`),
    UNIQUE KEY (`name`),
    UNIQUE KEY (`provider`, `subject`),
    FOREIGN KEY (`team_id`) REFERENCES `teams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `team_github_accounts` (
    `id` VARCHAR(36) NOT NULL,
    `team_id` VARCHAR(36) NOT NULL,
    `github_id` VARCHAR(255) NOT NULL,
    PRIMARY KEY (`id`),
    FOREIGN KEY (`team_id`) REFERENCES `teams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `sessions` (
    `id` VARCHAR(50) NOT NULL,
    `user_id` VARCHAR(36) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `expired_at` TIMESTAMP NOT NULL,
    PRIMARY KEY (`id`),
    FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- チームに割り当てたサーバー一式。状態はプロバイダー（CloudFormation のスタック）から読む。
CREATE TABLE IF NOT EXISTS `environments` (
    `id` VARCHAR(36) NOT NULL,
    `team_id` VARCHAR(36) NOT NULL,
    `provider_environment_id` VARCHAR(255) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP NULL DEFAULT NULL,
    -- 削除されていないチーム環境だけが値を持つ。一意制約で、チームごとに高々 1 つにする。
    `active_team_id` VARCHAR(36) AS (IF(`deleted_at` IS NULL, `team_id`, NULL)) STORED,
    PRIMARY KEY (`id`),
    UNIQUE KEY (`active_team_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `instances` (
    `id` VARCHAR(36) NOT NULL,
    `provider_instance_id` VARCHAR(255) NOT NULL,
    `team_id` VARCHAR(36) NOT NULL,
    `environment_id` VARCHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP NULL DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY (`environment_id`, `name`),
    FOREIGN KEY (`environment_id`) REFERENCES `environments` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `benchmarks` (
    `id` VARCHAR(36) NOT NULL,
    `instance_id` VARCHAR(36) NOT NULL,
    `team_id` VARCHAR (36) NOT NULL,
    `user_id` VARCHAR(36) NOT NULL,
    `status` ENUM('waiting', 'readying', 'running', 'finished') NOT NULL DEFAULT 'waiting',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `started_at` TIMESTAMP,
    `finished_at` TIMESTAMP,
    `score` BIGINT NOT NULL DEFAULT 0,
    `result` ENUM('passed', 'failed', 'error'),
    `error_message` TEXT,
    PRIMARY KEY (`id`),
    FOREIGN KEY (`instance_id`) REFERENCES `instances` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `benchmark_logs` (
    `benchmark_id` VARCHAR(36) NOT NULL,
    `user_log` MEDIUMTEXT NOT NULL,
    `admin_log` MEDIUMTEXT NOT NULL,
    PRIMARY KEY (`benchmark_id`),
    FOREIGN KEY (`benchmark_id`) REFERENCES `benchmarks` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `documents` (
    `id` VARCHAR(36) NOT NULL,
    `body` TEXT NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ポータルの DB が扱う問題。DB は 1 つの問題だけに使うので、行は高々 1 つになる。
CREATE TABLE IF NOT EXISTS `problems` (
    `id` TINYINT UNSIGNED NOT NULL DEFAULT 1,
    `name` VARCHAR(255) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    CHECK (`id` = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
