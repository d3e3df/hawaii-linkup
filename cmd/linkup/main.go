package main

import (
	"fmt"
	"log"

	"hawaii-linkup/internal/repository/memory"
	"hawaii-linkup/internal/service/discovery"
	"hawaii-linkup/internal/service/feed"
)

func main() {
	if err := runDiscovery(memory.NewDemoStore()); err != nil {
		log.Fatalf("discovery: %v", err)
	}
	if err := runFeed(memory.NewDemoStore()); err != nil {
		log.Fatalf("feed: %v", err)
	}
}

func runDiscovery(store *memory.Store) error {
	fmt.Println("=== A. Discovery ===")
	svc := discovery.NewService(store)

	c := discovery.Criteria{City: "Москва", MinAge: 20, MaxAge: 30, Interests: []string{"music"}}
	found, err := svc.Find("u1", c)
	if err != nil {
		return err
	}

	fmt.Println("Кандидаты для Анны (Москва, 20–30, music):")
	if len(found) == 0 {
		fmt.Println("  подходящих кандидатов нет")
	}
	for _, u := range found {
		fmt.Printf("  %s, %d, %s\n", u.Name, u.Age, u.City)
	}
	fmt.Println()
	return nil
}

func runFeed(store *memory.Store) error {
	fmt.Println("=== B. Feed ===")
	svc := feed.NewService(store)

	candidates, err := svc.Feed("u1", 10)
	if err != nil {
		return err
	}

	fmt.Println("Лента Анны (без заблокированных и уже лайкнутых):")
	if len(candidates) == 0 {
		fmt.Println("  кандидатов не осталось")
	}
	for _, u := range candidates {
		fmt.Printf("  %s, %d, %s\n", u.Name, u.Age, u.City)
	}
	fmt.Println()
	return nil
}
