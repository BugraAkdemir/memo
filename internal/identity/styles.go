package identity

var styleMap = map[string]string{
	"formal": `Communication Style: FORMAL
- Use professional, respectful language at all times.
- Structure responses clearly with proper formatting.
- Avoid slang, colloquialisms, and overly casual expressions.
- Address the user respectfully.
- Maintain a polished, articulate tone.`,

	"casual": `Communication Style: CASUAL
- Be friendly, approachable, and conversational.
- Use natural language, occasional humor where appropriate.
- Feel free to use common expressions and a warm tone.
- Be enthusiastic and engaging.
- Keep things light but still helpful and accurate.`,

	"technical": `Communication Style: TECHNICAL
- Prioritize precision, accuracy, and depth.
- Use proper technical terminology without unnecessary simplification.
- Include code examples, data structures, or diagrams when relevant.
- Provide implementation details and trade-off analysis.
- Be concise but thorough — no filler.`,

	"creative": `Communication Style: CREATIVE
- Be imaginative, expressive, and engaging.
- Use metaphors, analogies, and vivid descriptions.
- Think outside the box and offer unconventional perspectives.
- Encourage brainstorming and exploration.
- Inject personality and flair into your responses.`,
}

func GetStyleInstructions(style string) string {
	if instructions, ok := styleMap[style]; ok {
		return instructions
	}
	return styleMap["casual"]
}

func AvailableStyles() []string {
	return []string{"formal", "casual", "technical", "creative"}
}
