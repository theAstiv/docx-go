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

FROM mcr.microsoft.com/dotnet/runtime:8.0-alpine@sha256:a088dd404d4b1c7f7cad246d665eccfd5c05780a99a237a8643063d72dca2b0f
WORKDIR /app
COPY --from=build /app .
ENTRYPOINT ["dotnet", "validator.dll"]
