# OOXML Validator container
# .NET 8 + DocumentFormat.OpenXml + thin JSON wrapper
# Pin the SDK version so CI doesn't break on .NET updates.
FROM mcr.microsoft.com/dotnet/sdk:8.0-alpine AS build
WORKDIR /src
COPY validator.csproj .
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /app

FROM mcr.microsoft.com/dotnet/runtime:8.0-alpine
WORKDIR /app
COPY --from=build /app .
ENTRYPOINT ["dotnet", "validator.dll"]
