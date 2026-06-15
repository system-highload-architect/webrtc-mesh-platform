package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// bootstrapScyllaTables имитирует персистентный коммит DDL/DML в файлы таблиц ScyllaDB
func (s *SprStorageService) bootstrapScyllaTables() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Заводим b2b-паспорта в оперативную память кластера
	s.profiles["user_david"] = &SubscriberProfile{
		UserID:   "user_david",
		Name:     "Давид (Модератор)",
		Email:    "david@clearway.ru",
		Password: "admin123",
		Role:     "ORGANIZER",
	}

	s.profiles["user_employee"] = &SubscriberProfile{
		UserID:   "user_employee",
		Name:     "Константин (Участник)",
		Email:    "konstantin@clearway.ru",
		Password: "user123",
		Role:     "EMPLOYEE",
	}

	// Имитируем запись файлов коммит-лога (.db сегменты) на дисковый массив NVMe
	tableFile := filepath.Join(s.dbPath, "subscribers_replicated.db")
	f, err := os.OpenFile(tableFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		s.log.Error(fmt.Sprintf("Крах инициализации NoSQL таблиц ScyllaDB: %v", err))
		return
	}
	defer f.Close()

	for _, p := range s.profiles {
		line := fmt.Sprintf("%s;%s;%s;%s;%s\n", p.UserID, p.Name, p.Email, p.Password, p.Role)
		_, _ = f.Write([]byte(line))
	}
	s.log.Info("ScyllaDB СИМУЛЯЦИЯ -> Системные b2b-паспорта персистентно сохранены в кластере SPR.")
}
