#!/bin/bash

echo "Finding source packages for missing RPMs..."
echo "Missing Package Source RPM Mapping" > source_packages_fedora42_seed.txt
echo "==================================" >> source_packages_fedora42_seed.txt
echo "" >> source_packages_fedora42_seed.txt

# Create a temporary script with all the repoquery commands
echo "#!/bin/bash" > query_commands.sh
echo "dnf check-update >/dev/null 2>&1" >> query_commands.sh

while read -r line; do
    # Skip empty lines and comments
    [[ -z "$line" ]] && continue
    [[ $line == \#* ]] && continue
    
    # First, print the package name
    echo "echo \"$line\"" >> query_commands.sh
    # Then print a tab and the source package on the next line
    echo "echo -n -e \"\t\"" >> query_commands.sh
    echo "dnf repoquery --qf '%{sourcerpm}' $line 2>/dev/null | sed 's/\.src\.rpm$//' || echo 'Not Found'" >> query_commands.sh
    # Add a blank line between entries
    echo "echo \"\"" >> query_commands.sh
done < seed_packages.txt

# Make the script executable
chmod +x query_commands.sh

# Run all queries in a single container
docker run --rm -v "$(pwd)/query_commands.sh:/query_commands.sh" fedora:42 /query_commands.sh >> source_packages_fedora42_seed.txt

# Clean up
rm query_commands.sh

echo -e "\nResults saved in source_packages_fedora42_seed.txt"