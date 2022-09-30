#! /bin/bash

QUERY='(.spec.descriptor.component.repositoryContexts.[] | select(.).baseUrl)'
REPO=$(yq e "$QUERY" ./template.yaml)
# shellcheck disable=SC2001
NEW_REPO=$(echo "$REPO" | sed -e 's/\:[0-9]*/:5000/g')
echo "$NEW_REPO"
if [[ $(yq eval .spec.target ./template.yaml) == "control-plane" ]];
then
        yq -i e "$QUERY |= \"$NEW_REPO\"" ./template.yaml
fi
