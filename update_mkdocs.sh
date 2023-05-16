#!/bin/bash

filename="mkdocs.yml"
start_line=$(sed -n '/CHANGELOG:/=' "$filename")

changelogs_from_dir=$(ls ./docs/CHANGELOG) 
changelogs_from_mkdocs_file=$(grep -Eo "CHANGELOG/changelog-[0-9]+\.[0-9]+\.x\.md" mkdocs.yml | awk -F'/' '{print $2}')

changelogs_mkdocs_arr=( $changelogs_from_mkdocs_file )
changelogs_dir_arr=( $changelogs_from_dir )

new_changelog_file=""

# check if all files from CHANGELOG dir are present in mkdocs.yml
for value in "${changelogs_dir_arr[@]}"; do
    if [[ ! " ${changelogs_mkdocs_arr[@]} " =~ " $value " ]]; then
        # the value of the new_changelog_file will be name of the changelog file, which isn't in mkdocs.yml nav
        new_changelog_file=$value
    fi
done

# add new navigation entry only, when there is a new changelog file
if [[ -n "$new_changelog_file" ]]; then
    echo "Found new changelog file"

    version=$(echo $new_changelog_file | grep -oE [0-9]+\.[0-9]+)

    navigation_entry="Claudie v$version: CHANGELOG/$new_changelog_file"

    new_entry_line=$(($start_line + ${#changelogs_dir_arr[@]}))

    sed -i "${new_entry_line}i\\
    - ${navigation_entry}" $filename

    # commit and push
    git commit -am "add new changelog file to mkdocs.yml"

    git push

    echo "Altered mkdocs.yml with new CHANGELOG navigation entry was commited and pushed to origin"
else
    echo "There isn't a new changelog file"
fi