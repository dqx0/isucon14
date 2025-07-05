package main

import (
    "database/sql"
    "errors"
    "net/http"
)
// internal_handlers.goの排他制御を強化
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    for i := 0; i < 10; i++ { // 処理件数を減らして安全性を向上
        tx, err := db.BeginTxx(ctx, nil)
        if err != nil {
            break
        }
        defer tx.Rollback()

        // 最も古い未マッチのライドを取得（ロック付き）
        ride := &Ride{}
        if err := tx.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1 FOR UPDATE`); err != nil {
            tx.Rollback()
            if errors.Is(err, sql.ErrNoRows) {
                break
            }
            break
        }

        // 最も近い利用可能な椅子を探す（ロック付き）
        matched := &Chair{}
        if err := tx.GetContext(ctx, matched, `
            SELECT c.* FROM chairs c
            WHERE c.is_active = TRUE 
            AND c.id NOT IN (
                SELECT r.chair_id FROM rides r
                WHERE r.chair_id IS NOT NULL 
                AND r.evaluation IS NULL
                AND r.created_at >= DATE_SUB(NOW(), INTERVAL 1 HOUR)
            )
            ORDER BY (
                SELECT (ABS(cl.latitude - ?) + ABS(cl.longitude - ?))
                FROM chair_locations cl
                WHERE cl.chair_id = c.id
                ORDER BY cl.created_at DESC
                LIMIT 1
            ) ASC
            LIMIT 1
            FOR UPDATE
        `, ride.PickupLatitude, ride.PickupLongitude); err != nil {
            tx.Rollback()
            if errors.Is(err, sql.ErrNoRows) {
                break
            }
            break
        }

        // 二重チェック
        var count int
        if err := tx.GetContext(ctx, &count, `
            SELECT COUNT(*) FROM rides 
            WHERE chair_id = ? AND evaluation IS NULL
        `, matched.ID); err != nil || count > 0 {
            tx.Rollback()
            continue
        }

        // ライドに椅子をアサイン
        if _, err := tx.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matched.ID, ride.ID); err != nil {
            tx.Rollback()
            continue
        }

        if err := tx.Commit(); err != nil {
            continue
        }
    }

    w.WriteHeader(http.StatusNoContent)
}
