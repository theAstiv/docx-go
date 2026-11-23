# OOXML Validator container
# .NET 8 + DocumentFormat.OpenXml + thin JSON wrapper
# Base images are pinned by digest (the tag is for readers), so CI builds
# the same image every time; Dependabot proposes digest updates.
FROM mcr.microsoft.com/dotnet/sdk:8.0-alpine@sha256:4f4992f8f302150e4adb5e78c324731e968493a30c0638985ba29cb42a858cd6 AS build
WORKDIR /src
COPY validator.csproj .
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /app

FROM mcr.microsoft.com/dotnet/runtime:11.0-alpine@sha256:f79ce0f256d44492ac098aa861836d1a71b3b178019d7357cc6d1437f3ad78d1
WORKDIR /app
COPY --from=build /app .
ENTRYPOINT ["dotnet", "validator.dll"]
