CREATE TABLE tenants (
    tenant_id UUID PRIMARY KEY, -- 主キー制約 (暗黙的に NOT NULL かつ UNIQUE)
    name VARCHAR(255) NOT NULL UNIQUE, -- NOT NULL制約, 一意制約 (テナント名は必須かつ重複不可)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, -- NOT NULL制約, デフォルト制約
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, -- NOT NULL制約, デフォルト制約
    deleted_at TIMESTAMP NULL -- NULL許容 (論理削除用)
    -- その他のカラム...
);

CREATE TABLE words (
    word_id UUID PRIMARY KEY, -- 主キー制約
    tenant_id UUID NOT NULL, -- NOT NULL制約
    term VARCHAR(255) NOT NULL, -- NOT NULL制約 (単語は必須)
    definition TEXT NOT NULL, -- NOT NULL制約 (定義/答えも必須)
    deleted_at TIMESTAMP NULL, -- NULL許容 (論理削除用)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- その他のカラム...

    -- 外部キー制約
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE
    -- ↑ tenant_idはtenantsテーブルに存在する値しか許可しない
    -- CREATE INDEX idx_words_deleted_at ON words(deleted_at)
    -- ON DELETE CASCADE: 親テナントが削除されたら、関連する単語も自動的に削除する (要件に応じて NO ACTION, SET NULL なども検討)

    -- 一意制約 (オプション: 同じテナント内で同じ単語の重複を許さない場合)
    -- UNIQUE (tenant_id, term) -- このままだと論理削除しても同じ単語は追加できない
    -- 注意: この単純なUNIQUE制約では、is_deleted=true のものがあっても重複とみなされます。
    -- 有効な単語(is_deleted=false)の間でのみ一意性を保ちたい場合は、
    -- DBが対応していればFiltered Index/Partial Indexを使うか、アプリケーション側でのチェックが必要です。
    -- 例 (PostgreSQL): CREATE UNIQUE INDEX unique_active_word_per_tenant ON words (tenant_id, term) WHERE is_deleted = false;
);

CREATE TABLE learning_progress (
    progress_id UUID PRIMARY KEY, -- 主キー制約
    tenant_id UUID NOT NULL, -- NOT NULL制約
    word_id UUID NOT NULL, -- NOT NULL制約
    level INTEGER NOT NULL DEFAULT 1, -- NOT NULL制約, デフォルト制約 (デフォルトレベル1)
    next_review_date DATE NOT NULL, -- NOT NULL制約 (必ず設定される想定)
    last_reviewed_at TIMESTAMP NULL, -- NULL許容 (初回レビュー前)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- 外部キー制約
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    FOREIGN KEY (word_id) REFERENCES words(word_id) ON DELETE CASCADE,
    -- ↑ word_idはwordsテーブルに存在する値しか許可しない
    -- ON DELETE CASCADE: 親単語が削除されたら、学習進捗も削除する

    -- 一意制約: 同じテナントの同じ単語に対する進捗は一つだけ
    UNIQUE (tenant_id, word_id),

    -- CHECK制約: levelが指定された範囲内にあることを保証
    CHECK (level IN (1, 2, 3))
    -- または CHECK (level >= 1 AND level <= 3)
);