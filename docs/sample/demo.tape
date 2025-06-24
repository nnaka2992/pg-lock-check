# VHS script for pg-lock-check demo
# Run with: vhs docs/sample/demo.tape

Output docs/sample/demo.gif

Set FontSize 16
Set Width 1200
Set Height 800
Set Theme "Dracula"

Type "# pg-lock-check - Stop PostgreSQL Locks Before They Stop You 🔒"
Enter
Sleep 1s
Ctrl+L

Type "# Let's analyze a dangerous migration: docs/sample/001_add_uuid.sql"
Enter
Sleep 1s
Type ""
Enter
Type "cat docs/sample/001_add_uuid.sql"
Enter
Sleep 1s
Ctrl+L

Type "# Feature 1: Safe Migration Suggestions 💡"
Enter
Sleep 1s
Type "# pg-lock-check detects the danger and suggests a safer approach"
Enter
Sleep 1s
Type ""
Enter
Type "pg-lock-check -f docs/sample/001_add_uuid.sql"
Enter
Sleep 4s
Ctrl+L

Type "# Feature 2: Disable Suggestions with --no-suggestion 🎭"
Enter
Sleep 1s
Type "# Sometimes you just want the analysis without suggestions"
Enter
Sleep 1s
Type ""
Enter
Type "pg-lock-check --no-suggestion -f docs/sample/001_add_uuid.sql"
Enter
Sleep 3s
Ctrl+L

Type "# Feature 3: CI/CD Integration with JSON Output 🚀"
Enter
Sleep 1s
Type "# Parse the output in your CI/CD pipeline to block dangerous migrations"
Enter
Sleep 1s
Type ""
Enter
Type "pg-lock-check -f docs/sample/001_add_uuid.sql -o json | jq '.results[0]'"
Enter
Sleep 3s
Type ""
Enter
Type "# Extract just the severity for automated decisions:"
Enter
Type "pg-lock-check -f docs/sample/001_add_uuid.sql -o json | jq -r '.results[0].severity'"
Enter
Sleep 2s
Type ""
Enter
Type "# Block CRITICAL operations in your pipeline! 🎯"
Enter
Sleep 2s
