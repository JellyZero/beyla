FROM mcr.microsoft.com/dotnet/sdk:latest

# Set the working directory to /build
WORKDIR /build

# Copy the source code into the image for building
COPY test/ test/

WORKDIR /build/test/integration/components/dotnetserver

EXPOSE 5266

# Run the .net app
CMD [ "dotnet", "run" ]
