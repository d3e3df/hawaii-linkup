package main

import (
	"fmt"
	"log"

	"hawaii-linkup/internal/repository/memory"
	"hawaii-linkup/internal/service/eligibility"
)

func main() {
	if err := runEligibility(memory.NewDemoStore()); err != nil {
		log.Fatalf("eligibility: %v", err)
	}
}

// runEligibility - контракт C: проверка допустимости кандидата
func runEligibility(store *memory.Store) error {
	fmt.Println("=== C. Eligibility ===")
	svc := eligibility.NewService(store)

	pairs := [][2]string{
		{"u1", "u2"}, // можно
		{"u1", "u4"}, // блокировка
		{"u1", "u5"}, // неактивный профиль
		{"u1", "u6"}, // уже лайкнут
		{"u1", "u1"}, // сам себя
	}
	for _, p := range pairs {
		d, err := svc.CanLike(p[0], p[1])
		if err != nil {
			return err
		}
		fmt.Printf("%s -> %s: allowed=%v, reason=%s\n", p[0], p[1], d.Allowed, d.Reason)
	}

	_, err := svc.CanLike("u1", "nobody")
	fmt.Println("несуществующий пользователь:", err)
	fmt.Println()
	return nil
}
