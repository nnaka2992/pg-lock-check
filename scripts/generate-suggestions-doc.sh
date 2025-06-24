#!/bin/bash

set -e

# File paths
YAML_FILE="internal/suggester/suggestions.yaml"
TEMPLATE_FILE="docs/design/suggestions.template.md"
OUTPUT_FILE="docs/design/suggestions.md"

# Extract metadata from YAML
VERSION=$(yq '.version' "$YAML_FILE")
GENERATED_AT=$(date -u +"%Y-%m-%d")

# Count operations
WITH_ALTERNATIVES=$(yq '.operations_with_alternatives | length' "$YAML_FILE")
WITHOUT_ALTERNATIVES=$(yq '.operations_without_alternatives | length' "$YAML_FILE")
TOTAL_OPERATIONS=$((WITH_ALTERNATIVES + WITHOUT_ALTERNATIVES))

# Calculate percentages
WITH_ALTERNATIVES_PERCENT=$((WITH_ALTERNATIVES * 100 / TOTAL_OPERATIONS))
WITHOUT_ALTERNATIVES_PERCENT=$((WITHOUT_ALTERNATIVES * 100 / TOTAL_OPERATIONS))

# Generate table for operations with alternatives
OPERATIONS_WITH_ALTERNATIVES_TABLE=""
count=$(yq eval '.operations_with_alternatives | length' "$YAML_FILE")

for ((idx=0; idx<$count; idx++)); do
    operation=$(yq eval ".operations_with_alternatives[$idx].operation" "$YAML_FILE")
    category=$(yq eval ".operations_with_alternatives[$idx].category" "$YAML_FILE")
    
    # Get all step descriptions as a single line
    steps=$(yq eval ".operations_with_alternatives[$idx].steps[].description" "$YAML_FILE" | tr '\n' '; ' | sed 's/; $//')
    
    # Get transaction safety
    trans_safe_values=$(yq eval ".operations_with_alternatives[$idx].steps[].can_run_in_transaction" "$YAML_FILE" | sort -u)
    
    if [ "$(echo "$trans_safe_values" | wc -l)" -eq 1 ]; then
        if [ "$trans_safe_values" = "true" ]; then
            trans_status="✅ Yes"
        else
            trans_status="❌ No"
        fi
    else
        trans_status="⚠️ Mixed"
    fi
    
    OPERATIONS_WITH_ALTERNATIVES_TABLE="${OPERATIONS_WITH_ALTERNATIVES_TABLE}| $operation | $category | $steps | $trans_status |"$'\n'
done

# Remove trailing newline
OPERATIONS_WITH_ALTERNATIVES_TABLE="${OPERATIONS_WITH_ALTERNATIVES_TABLE%$'\n'}"

# Export all variables for envsubst
export VERSION GENERATED_AT TOTAL_OPERATIONS WITH_ALTERNATIVES WITHOUT_ALTERNATIVES
export WITH_ALTERNATIVES_PERCENT WITHOUT_ALTERNATIVES_PERCENT OPERATIONS_WITH_ALTERNATIVES_TABLE

# Generate the documentation
envsubst < "$TEMPLATE_FILE" > "$OUTPUT_FILE"

echo "Generated $OUTPUT_FILE with $TOTAL_OPERATIONS operations ($WITH_ALTERNATIVES with alternatives)"