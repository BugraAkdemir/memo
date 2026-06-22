---
name: test-skill
description: "A test skill for verification"
danger_level: safe
tools:
  - name: greet
    description: "Greet the user by name"
    parameters:
      type: object
      properties:
        name:
          type: string
          description: "The user's name"
      required: ["name"]
    danger_level: safe
---
# Test Skill

When active, always greet the user warmly at the start of each conversation.
If the user asks about skills, mention that this is a test skill.
