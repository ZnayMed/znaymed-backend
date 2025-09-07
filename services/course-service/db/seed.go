package db

import (
	"gorm.io/gorm"
)

func seedInitialData(db *gorm.DB) error {
	subjects := []Subject{
		{
			Title: "Математика",
			Sections: []Section{
				{
					Title:       "Алгебра",
					Description: "Основы алгебры: выражения, уравнения, функции",
					PriceKopeck: 100, // 199 руб
					Topics: []Topic{
						{Title: "Линейные уравнения", Description: "ax + b = 0", TgID: "tg_math_lin", MindmapURL: "https://mindmap.example.com/lin"},
						{Title: "Квадратные уравнения", Description: "ax² + bx + c = 0", TgID: "tg_math_quad", MindmapURL: "https://mindmap.example.com/quad"},
					},
				},
				{
					Title:       "Геометрия",
					Description: "Планиметрия и стереометрия",
					PriceKopeck: 100, // 249 руб
					Topics: []Topic{
						{Title: "Треугольники", Description: "Классификация и свойства", TgID: "tg_math_tri", MindmapURL: "https://mindmap.example.com/tri"},
					},
				},
			},
		},
		{
			Title: "Физика",
			Sections: []Section{
				{
					Title:       "Механика",
					Description: "Законы Ньютона, кинематика, динамика",
					PriceKopeck: 100,
					Topics: []Topic{
						{Title: "Третий закон Ньютона", Description: "F₁ = −F₂", TgID: "tg_phys_newton3", MindmapURL: "https://mindmap.example.com/n3"},
					},
				},
				{
					Title:       "Оптика",
					Description: "Свет, линзы, зеркала",
					PriceKopeck: 100,
					Topics: []Topic{
						{Title: "Линзы", Description: "Собирательные и рассеивающие", TgID: "tg_phys_lenses", MindmapURL: "https://mindmap.example.com/lens"},
					},
				},
			},
		},
		{
			Title: "Химия",
			Sections: []Section{
				{
					Title:       "Органическая химия",
					Description: "Углеводороды, функциональные группы",
					PriceKopeck: 100,
					Topics: []Topic{
						{Title: "Алканы", Description: "CnH₂n+2", TgID: "tg_chem_alkanes", MindmapURL: "https://mindmap.example.com/alk"},
					},
				},
				{
					Title:       "Неорганическая химия",
					Description: "Соли, оксиды, кислоты",
					PriceKopeck: 100,
					Topics: []Topic{
						{Title: "Кислоты", Description: "Сильные и слабые", TgID: "tg_chem_acids", MindmapURL: "https://mindmap.example.com/acid"},
					},
				},
			},
		},
	}

	// твой остальной код сидера
	return db.Create(&subjects).Error
}
