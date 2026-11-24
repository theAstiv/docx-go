# OOXML Validator container
# .NET 10 (LTS) + DocumentFormat.OpenXml + thin JSON wrapper
#
# Base images are pinned by digest (the tag is for readers), so CI builds
# the same image every time; Dependabot proposes digest updates.
#
# The SDK image, the runtime image and TargetFramework in validator.csproj
# must name the same .NET version: an app built for one major version does
# not start on another's runtime (exit 150, "You must install or update
# .NET"). Move all three together; Dependabot is told not to propose major
# bumps of these images (.github/dependabot.yml).
FROM mcr.microsoft.com/dotnet/sdk:10.0-alpine@sha256:0294f285d062afacc8b5f6d7febb0cd4ee879320e5585fa15f2f4ebf1feb9ba7 AS build
WORKDIR /src
COPY validator.csproj .
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /app

FROM mcr.microsoft.com/dotnet/runtime:10.0-alpine@sha256:7518aaad9e4328001518006a7e8b3fcf6ce8f8816355574f7264c1fa0a0ce188
WORKDIR /app
COPY --from=build /app .
ENTRYPOINT ["dotnet", "validator.dll"]
