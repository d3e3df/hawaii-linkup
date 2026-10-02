package main

import (
	"fmt"
	"log"

	"hawaii-linkup/internal/repository/memory"
	"hawaii-linkup/internal/service/discovery"
)

func main() {
	if err := runDiscovery(memory.NewDemoStore()); err != nil {
		log.Fatalf("discovery: %v", err)
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
