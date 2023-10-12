import java.io.BufferedReader;
import java.io.FileReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.security.Security;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

public class FIPS {

  public static void main(String[] args) {
    // Exit code 8 is used to differentiate valid scan returns.
    if (validateFipsHost())
      System.exit(8);

    if (validateSystemProperties())
      System.exit(8);

    if (validateAlgorithms(args))
      System.exit(8);
  }

  // validate that the tool is running on a FIPS enabled host
  public static boolean validateFipsHost() {
    String content = null;
    try {
      byte[] fileBytes = Files.readAllBytes(Paths.get("/proc/sys/crypto/fips_enabled"));
      content = new String(fileBytes, StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
    int fileInt = Integer.parseInt(content.trim());
    if (fileInt != 1) {
      System.err.println("java scan must be run on a FIPS enabled host");
      return true;
    }
    return false;
  }

  // https://access.redhat.com/documentation/en-us/openjdk/8/html/configuring_openjdk_8_on_rhel_with_fips/config-fips-in-openjdk
  public static boolean validateSystemProperties() {
    Boolean prop1 = validateSystemProperty("security.useSystemPropertiesFile", "false");
    Boolean prop2 = validateSystemProperty("java.security.disableSystemPropertiesFile", "true");
    Boolean prop3 = validateSystemProperty("com.redhat.fips", "false");
    if (prop1 || prop2 || prop3)
      return true;

    return false;
  }

  public static boolean validateSystemProperty(String property, String value) {
    String str = System.getProperty(property);
    if (str != null && str.equals(value)) {
      System.err.append("java property '" + property + "' should not be set to " + value + "\n");
      return true;
    }
    return false;
  }

  public static boolean validateAlgorithms(String[] args) {
    ArrayList<String> ar = new ArrayList<String>();
    if (args.length > 0) {
      BufferedReader reader;
      try {
        reader = new BufferedReader(new FileReader(args[0]));
        String line = null;
        while ((line = reader.readLine()) != null) {
          ar.add(line);
        }
        reader.close();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }

    String disabledAlgorithms[] = ar.toArray(new String[ar.size()]);
    List<String> disAlgor = Arrays.asList(Security.getProperty("jdk.tls.disabledAlgorithms").trim().split(","));
    List<String> trimmedDisAlgor = disAlgor.stream().map(String::trim).collect(Collectors.toList());
    System.out.println("jdk.tls.disabledAlgorithms: " + trimmedDisAlgor.toString());
    System.out.println("Total disabled algorithms = " + trimmedDisAlgor.size());

    List<String> reqDisAlgList = Arrays.asList(disabledAlgorithms);
    System.out.println("Checking for these disabled algorithms = " + reqDisAlgList.toString());
    System.out.println("Total algorithms being checked = " + reqDisAlgList.size());

    Boolean result = false;
    for (int i = 0; i < disabledAlgorithms.length; i++) {
      if (!trimmedDisAlgor.contains(disabledAlgorithms[i])) {
        System.err.append("algorithm '" + disabledAlgorithms[i] + "' should be disabled\n");
        result = true;
      }
    }
    return result;
  }
}
