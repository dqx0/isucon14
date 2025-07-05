package main

import (
	"database/sql"
	"errors"
	"net/http"
)

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // 一度に全ての未マッチライドを処理
    for {
        // 最も古い未マッチのライドを取得
        ride := &Ride{}
        if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                break // 未マッチのライドがない
            }
            writeError(w, http.StatusInternalServerError, err)
            return
        }

        // 現在進行中のライドで使用中でない椅子を探す
        matched := &Chair{}
        if err := db.GetContext(ctx, matched, `
            SELECT * FROM chairs 
            WHERE is_active = TRUE 
            AND id NOT IN (
                SELECT chair_id FROM rides 
                WHERE chair_id IS NOT NULL 
                AND evaluation IS NULL
            ) 
            LIMIT 1
        `); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                break // 利用可能な椅子がない
            }
            writeError(w, http.StatusInternalServerError, err)
            return
        }

        // ライドに椅子をアサイン
        if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matched.ID, ride.ID); err != nil {
            writeError(w, http.StatusInternalServerError, err)
            return
        }
    }

    w.WriteHeader(http.StatusNoContent)
}
