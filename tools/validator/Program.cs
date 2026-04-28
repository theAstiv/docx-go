// Thin OOXML validator wrapper.
// Usage: validator <file1.docx> [file2.docx ...]
// Outputs JSON: {"file":"...","errors":[...],"warnings":[...]}
// Exit code 0 = no errors; 1 = schema errors found; 2 = usage/IO error.
using System.Text.Json;
using DocumentFormat.OpenXml.Packaging;
using DocumentFormat.OpenXml.Validation;

if (args.Length == 0)
{
    Console.Error.WriteLine("usage: validator <file> [file ...]");
    return 2;
}

var validator = new OpenXmlValidator();
int exitCode = 0;

foreach (var path in args)
{
    var errors = new List<string>();
    var warnings = new List<string>();

    try
    {
        using var doc = WordprocessingDocument.Open(path, false);
        foreach (var err in validator.Validate(doc))
        {
            var msg = $"{err.ErrorType}: {err.Description} (path: {err.Path?.XPath})";
            if (err.ErrorType == ValidationErrorType.Schema)
                errors.Add(msg);
            else
                warnings.Add(msg);
        }
    }
    catch (Exception ex)
    {
        errors.Add($"open error: {ex.Message}");
    }

    var result = new { file = path, errors, warnings };
    Console.WriteLine(JsonSerializer.Serialize(result));

    if (errors.Count > 0)
        exitCode = 1;
}

return exitCode;
